package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ProxyName is the name of the proxy's pod
var ProxyName = os.Getenv("POD_NAME")

// ProxyNamespace is the namespace the proxy's pod resides in
var ProxyNamespace = os.Getenv("POD_NAMESPACE")

// ProxyStatefulSet is the StatefulSet the proxy's pod resides in
var ProxyStatefulSet = os.Getenv("POD_STATEFULSET")

// ProxyOrdinal is the proxy's pod's ordinal in the StatefulSet
var ProxyOrdinal = getProxyOrdinal(ProxyName)

var kubeClient *kubernetes.Clientset

// Info of StatefulSet
var proxies struct {
	Count   int64
	CountMu sync.Mutex
	List    struct {
		sync.RWMutex
		IPs     string
		Version string
	}
}

// State of the current proxy
var state struct {
	ActiveRequests   int64
	ActiveRequestsMu sync.Mutex
	RequestCounter   uint64
	DenyCounter      uint64
	IdleShutdown     struct {
		sync.RWMutex
		LastTime time.Time
	}
}

// Config from annotations (+ readiness probe)
var config struct {
	MinProxies    int64
	MaxProxies    int64
	MaxRequests   int64
	MaxLoadFactor float64
	ProxyTimeout  int64
	IdleTimeout   int64
	DebugLevel    int64

	// HTTP config comes from readiness probe
	HTTP struct {
		Path string
		Port int
	}
}

func debugPrint(level int, format string, args ...interface{}) {
	if int64(level) > config.DebugLevel {
		return
	}

	log.Printf(format, args...)
}

// Changes the StatefulSet replica count
func scaleStatefulSet(newScale int) {
	debugPrint(2, "[+] Attempting to scale to %v proxies", newScale)

	// Cap it at max proxies
	if int64(newScale) > config.MaxProxies {
		newScale = int(config.MaxProxies)
	}

	// Skip useless scalings
	if int64(newScale) == proxies.Count || int64(newScale) < config.MinProxies {
		return
	}

	retries := 5
	for retry := 0; retry < retries; retry++ {
		statefulSets := kubeClient.AppsV1().StatefulSets(ProxyNamespace)

		scale, err := statefulSets.GetScale(context.Background(), ProxyStatefulSet, metav1.GetOptions{})
		if err != nil {
			log.Fatalf("[!] Error getting StatefulSet's scale: %s", err)
		}

		scale.Spec.Replicas = int32(newScale)
		scale.Status.Replicas = int32(newScale)

		scale, err = statefulSets.UpdateScale(context.Background(), ProxyStatefulSet, scale, metav1.UpdateOptions{})
		if err != nil {
			debugPrint(1, "[!] Error updating StatefulSet's scale (try %v): %s", retry, err)
			continue
		}

		proxies.Count = int64(newScale)
		return
	}

	log.Fatalf("[!] Failed to scale up after %v tries", retries)
}

// Scales up if we are the last proxy and have not hit the max proxies
func scaleUp() bool {
	if proxies.Count+1 <= config.MaxProxies && ProxyOrdinal+1 == proxies.Count {
		proxies.CountMu.Lock()
		defer proxies.CountMu.Unlock()

		if proxies.Count+1 <= config.MaxProxies && ProxyOrdinal+1 == proxies.Count {
			scaleStatefulSet(int(ProxyOrdinal) + 2)
			return true
		}
	}

	return false
}

// Scales down if we are the last proxy and have not hit the min proxies
func scaleDown() bool {
	if proxies.Count-1 >= config.MinProxies && ProxyOrdinal+1 == proxies.Count {
		proxies.CountMu.Lock()
		defer proxies.CountMu.Unlock()

		if proxies.Count-1 >= config.MinProxies && ProxyOrdinal+1 == proxies.Count {
			scaleStatefulSet(int(ProxyOrdinal))
			return true
		}
	}

	return false
}

// Writes the current proxy's metrics to response writer
func writeProxyMetrics(w http.ResponseWriter, proxyStatus int) {
	if proxyStatus == http.StatusTooManyRequests {
		atomic.AddUint64(&state.DenyCounter, 1)
	}

	free := int(float64(config.MaxRequests)*config.MaxLoadFactor) - int(state.ActiveRequests)

	// If we have no more free requests based on load factor, try to scale up
	if free <= 0 {
		go scaleUp()
	}

	w.Header().Set("Proxy-Counter", strconv.Itoa(int(atomic.AddUint64(&state.RequestCounter, 1))))
	w.Header().Set("Proxy-Free", strconv.Itoa(free))
	w.Header().Set("Proxy-Ordinal", strconv.Itoa(int(ProxyOrdinal)))
	w.Header().Set("Proxy-Status", strconv.Itoa(proxyStatus))

	proxies.List.RLock()
	w.Header().Set("Proxy-Version", proxies.List.Version)
	w.Header().Set("Proxy-List", proxies.List.IPs)
	proxies.List.RUnlock()
}

// Proxy's HTTP handler
func httpHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Handle ensure requests
	if handleEnsureRequest(w, r) {
		return
	}

	// Forward-To is the host to forward the request to
	forwardTo := strings.TrimSpace(r.Header.Get("Forward-To"))

	// Is there no Forward-To header?
	if forwardTo == "" {
		// If so, return metrics.
		writeProxyMetrics(w, http.StatusOK)
		return
	}

	// Have we fully maxed out?
	if state.ActiveRequests >= int64(config.MaxRequests) {
		// If so, deny the request and return metrics
		writeProxyMetrics(w, http.StatusTooManyRequests)
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	// Perform a double check after locking
	state.ActiveRequestsMu.Lock()
	if state.ActiveRequests >= int64(config.MaxRequests) {
		state.ActiveRequestsMu.Unlock()
		writeProxyMetrics(w, http.StatusTooManyRequests)
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	// Increase the active request count
	atomic.AddInt64(&state.ActiveRequests, 1)
	debugPrint(3, "[>] Active Requests: %v", state.ActiveRequests)

	state.ActiveRequestsMu.Unlock()

	// Delete the Forward-To header for when we copy the request to proxy it
	r.Header.Del("Forward-To")

	// Read the body to copy it
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		atomic.AddInt64(&state.ActiveRequests, -1)
		writeProxyMetrics(w, http.StatusInternalServerError)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Create the proxy request
	proxyRequest, err := http.NewRequest(r.Method, forwardTo, bytes.NewReader(body))
	if err != nil {
		atomic.AddInt64(&state.ActiveRequests, -1)
		writeProxyMetrics(w, http.StatusInternalServerError)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Copy the headers
	proxyRequest.Header = r.Header

	// Do the actual request
	doAsyncProxyRequest(w, proxyRequest, strings.ToLower(strings.TrimSpace(r.Header.Get("Insecure-Skip-Verify"))) == "true")
}

// Handles an ensure request if it exists, returns false if none exists
func handleEnsureRequest(w http.ResponseWriter, r *http.Request) bool {
	ensure := strings.TrimSpace(r.Header.Get("Ensure-Requests"))
	if ensure == "" {
		return false
	}

	// Ensure-Requests are the number of requests to expect
	ensureRequests, err := strconv.ParseUint(ensure, 10, 64)
	if err != nil {
		writeProxyMetrics(w, http.StatusInternalServerError)
		return true
	}

	// Determine how many proxies are needed based on the ideal load amount for each proxy
	// Why does Go not have a min that works with ints?
	desiredProxyCount := int64(math.Min(float64(config.MaxProxies), float64(int64(ensureRequests)/int64(float64(config.MaxRequests)*config.MaxLoadFactor))))

	// Scale up, if necessary
	if proxies.Count < desiredProxyCount {
		proxies.CountMu.Lock()

		if proxies.Count < desiredProxyCount {
			scaleStatefulSet(int(desiredProxyCount))
		}

		proxies.CountMu.Unlock()
	}

	writeProxyMetrics(w, http.StatusOK)
	return true
}

// Does an async proxy request and returns the status code if returned before the timeout
func doAsyncProxyRequest(w http.ResponseWriter, proxyRequest *http.Request, insecureSkipVerify bool) {
	timeoutChan := make(chan bool, 2)

	var requestResponse *http.Response
	var requestResponseBody []byte
	var requestError error

	// Start the request
	go func() {
		defer func() {
			// Restart the timer
			resetIdleShutdown()

			// Decrement the current number of active requests
			atomic.AddInt64(&state.ActiveRequests, -1)
			debugPrint(3, "[<] Active requests: %v", state.ActiveRequests)
		}()

		// Do the request
		var httpClient http.Client
		if insecureSkipVerify {
			httpClient.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}

		requestResponse, requestError = httpClient.Do(proxyRequest)

		// Was there no error?
		if requestError == nil {
			defer requestResponse.Body.Close()

			// Read the body
			if body, err := ioutil.ReadAll(requestResponse.Body); err == nil {
				requestResponseBody = body
			} else {
				requestError = err

				debugPrint(2, "[!] Failed to read request to %v response body from: %v", proxyRequest.URL.String(), requestError)
			}
		} else {
			debugPrint(2, "[!] Request to %v failed: %v", proxyRequest.URL.String(), requestError)
		}

		// We did not timeout, request finished
		timeoutChan <- false
	}()

	// Start the timeout
	go func() {
		// Sleep for the timeout then notify the timeout channel
		time.Sleep(time.Duration(config.ProxyTimeout) * time.Millisecond)
		timeoutChan <- true
	}()

	if <-timeoutChan {
		// We did timeout, request still being processed
		writeProxyMetrics(w, http.StatusAccepted)
		w.WriteHeader(http.StatusAccepted)
	} else {
		// We did not timeout, try to copy the response back
		if requestError == nil {
			// Clear headers
			for k := range w.Header() {
				w.Header().Del(k)
			}

			// Copy headers
			for k, values := range requestResponse.Header {
				for _, v := range values {
					w.Header().Add(k, v)
				}
			}

			writeProxyMetrics(w, http.StatusOK)
			w.WriteHeader(requestResponse.StatusCode)

			w.Write(requestResponseBody)
		} else {
			// The request entirely failed
			writeProxyMetrics(w, http.StatusInternalServerError)
			w.WriteHeader(http.StatusInternalServerError)

			// Send error in body
			w.Write([]byte(requestError.Error()))
		}
	}
}

// Starts the HTTP server
func startServer() {
	http.HandleFunc(config.HTTP.Path, httpHandler)

	debugPrint(1, "[+] Listening on port %v (path \"%v\")", config.HTTP.Port, config.HTTP.Path)
	log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%v", config.HTTP.Port), nil))
}

// Returns the proxy's ordinal, which represents the proxy's current index in the StatefulSet
func getProxyOrdinal(podName string) int64 {
	value, err := strconv.ParseUint(strings.Replace(podName, ProxyStatefulSet+"-", "", 1), 10, 64)
	if err != nil {
		log.Fatalf("[!] Failed to parse proxy ordinal in %s: %v", podName, err)
	}

	return int64(value)
}

// Resets the idle shutdown timer if applicable
func resetIdleShutdown() {
	state.IdleShutdown.LastTime = time.Now()
}

// Should we do an idle shutdown?
func shouldDoIdleShutdown() bool {
	return ProxyOrdinal != 0 && ProxyOrdinal >= config.MinProxies && state.ActiveRequests == 0 && time.Since(state.IdleShutdown.LastTime) >= time.Duration(config.IdleTimeout)*time.Second
}

// Sets up the idle shutdown timer
func setupIdleShutdown() {
	// Never scale to 0
	if ProxyOrdinal == 0 {
		return
	}

	resetIdleShutdown()

	// Waits for the idle shutdown timer to finish
	go func() {
		for {
			if shouldDoIdleShutdown() {
				state.IdleShutdown.Lock()

				if shouldDoIdleShutdown() && scaleDown() {
					// Not necessary, but this proxy is going to shutdown anyway
					os.Exit(0)
				}

				state.IdleShutdown.Unlock()
			}

			time.Sleep(time.Second)
		}
	}()
}

// Updates the HTTP config from any containers valid readiness probe
func updateHTTPConfig(containers []corev1.Container) error {
	// Don't update if already valid
	if config.HTTP.Port != 0 {
		return nil
	}

	for _, c := range containers {
		if readinessProbe := c.ReadinessProbe; readinessProbe != nil {
			if httpGet := readinessProbe.HTTPGet; httpGet != nil {
				config.HTTP.Path = httpGet.Path
				config.HTTP.Port = httpGet.Port.IntValue()
				break
			}
		}
	}

	if config.HTTP.Port == 0 {
		return fmt.Errorf("found no valid HTTP get readiness probe in container spec")
	}

	if config.HTTP.Path == "" {
		config.HTTP.Path = "/"
	}

	return nil
}

// Updates the proxy list from the StatefulSet
func updateProxyList(set *v1.StatefulSet) error {
	// Get the pods that are part of the current StatefulSet
	podList, err := kubeClient.CoreV1().Pods(ProxyNamespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "component=" + ProxyStatefulSet,
	})

	if err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		return fmt.Errorf("found no pods in the StatefulSet %v", ProxyStatefulSet)
	}

	// Sort the proxies in increasing ordinal number
	sort.Slice(podList.Items, func(i, j int) bool {
		podA := podList.Items[i]
		podB := podList.Items[j]

		return getProxyOrdinal(podA.Name) < getProxyOrdinal(podB.Name)
	})

	if err := updateHTTPConfig(podList.Items[0].Spec.Containers); err != nil {
		return err
	}

	// Determine which pods are ready
	var newProxyList strings.Builder
	newProxyList.WriteRune('{')

	// Construct the list of pods that are running and pass the readiness check
	for ordinal, pod := range podList.Items {
		readinessCheck := false

		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				readinessCheck = true
				break
			}
		}

		if readinessCheck && pod.Status.Phase == corev1.PodRunning {
			if newProxyList.Len() != 1 {
				newProxyList.WriteRune(',')
			}

			newProxyList.WriteString(fmt.Sprintf(`"%v":"%v"`, ordinal, pod.Status.PodIP))
		}
	}

	newProxyList.WriteRune('}')

	// Update the active proxies list
	proxies.List.Lock()
	proxies.List.IPs = newProxyList.String()
	proxies.List.Version = set.ObjectMeta.ResourceVersion
	proxies.List.Unlock()

	// Update the number of intended proxies
	newProxyCount := int64(*set.Spec.Replicas)

	// Begin shared lock for idle shutdown
	state.IdleShutdown.RLock()
	if oldProxyCount := atomic.SwapInt64(&proxies.Count, int64(newProxyCount)); oldProxyCount != newProxyCount {
		// If we downscaled, retrigger shutdown timer
		if oldProxyCount > newProxyCount {
			resetIdleShutdown()
		}

		debugPrint(2, "[+] New proxy count: %v", proxies.Count)
	}
	state.IdleShutdown.RUnlock()

	return nil
}

func getOptionalConfigValue(annotations map[string]string, configName string, defaultValue uint64) (uint64, error) {
	stringValue, ok := annotations[configName]
	stringValue = strings.TrimSpace(stringValue)

	if !ok || stringValue == "" {
		debugPrint(1, "[+] Defaulting %v to %v", configName, defaultValue)
		return defaultValue, nil
	}

	value, err := strconv.ParseUint(stringValue, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%v was not properly defined: %v", configName, err)
	}

	return value, nil
}

func getOptionalConfigValueFloat(annotations map[string]string, configName string, defaultValue float64) (float64, error) {
	stringValue, ok := annotations[configName]
	stringValue = strings.TrimSpace(stringValue)

	if !ok || stringValue == "" {
		debugPrint(1, "[+] Defaulting %v to %v", configName, defaultValue)
		return defaultValue, nil
	}

	value, err := strconv.ParseFloat(stringValue, 64)
	if err != nil {
		return 0, fmt.Errorf("%v was not properly defined: %v", configName, err)
	}

	return value, nil
}

// Updates the proxy config from the annotations
func updateProxyConfig(annotations map[string]string) error {
	// config.MinProxies is the minimum number of proxy pods
	newMinProxies, err := getOptionalConfigValue(annotations, "minProxies", 1)
	if err != nil {
		return err
	}

	// config.MinProxies must be > 0
	if newMinProxies == 0 {
		return fmt.Errorf("minProxies must be > 0: %v", err)
	}

	// config.MaxProxies is the maximum number of proxy pods
	newMaxProxies, err := getOptionalConfigValue(annotations, "maxProxies", math.MaxInt64)
	if err != nil {
		return err
	}

	// config.MaxProxies must be >= config.MinProxies
	if newMaxProxies < newMinProxies {
		return fmt.Errorf("maxProxies must be >= minProxies: %v was not >= %v", newMaxProxies, newMinProxies)
	}

	// config.MaxRequests represents the actual maximum requests a proxy can handle
	newMaxRequests, err := getOptionalConfigValue(annotations, "maxRequests", 100)
	if err != nil {
		return err
	}

	// config.MaxLoadFactor represents the ideal max requests a proxy should handle (target = config.MaxLoadFactor * config.MaxRequests)
	newMaxLoadFactor, err := getOptionalConfigValueFloat(annotations, "maxLoadFactor", 0.5)
	if err != nil {
		return err
	}

	// config.ProxyTimeout is timeout in milliseconds the proxy waits for a response from the original host before moving on
	newProxyTimeout, err := getOptionalConfigValue(annotations, "proxyTimeout", 100)
	if err != nil {
		return err
	}

	// config.IdleTimeout is time in seconds the proxy waits for to shutdown after no activity
	newIdleTimeout, err := getOptionalConfigValue(annotations, "idleTimeout", 10)
	if err != nil {
		return err
	}

	// config.DebugLevel is the debug verbosity
	newDebugLevel, err := getOptionalConfigValue(annotations, "debugLevel", 0)
	if err != nil {
		return err
	}

	// Begin shared lock for idle shutdown
	state.IdleShutdown.RLock()
	defer state.IdleShutdown.RUnlock()

	// Determine if we should restart the idle timer
	if int64(newIdleTimeout) != config.IdleTimeout || (ProxyOrdinal > int64(newMinProxies) && ProxyOrdinal <= config.MinProxies) {
		resetIdleShutdown()
	}

	// Update the config globals
	config.MinProxies = int64(newMinProxies)
	config.MaxProxies = int64(newMaxProxies)
	config.MaxRequests = int64(newMaxRequests)
	config.MaxLoadFactor = newMaxLoadFactor
	config.ProxyTimeout = int64(newProxyTimeout)
	config.IdleTimeout = int64(newIdleTimeout)
	config.DebugLevel = int64(newDebugLevel)

	// If we are the last proxy, ensure the min/max number of proxies
	if ProxyOrdinal+1 == proxies.Count {
		proxies.CountMu.Lock()

		// Double check after locking
		if ProxyOrdinal+1 == proxies.Count {
			if proxies.Count < config.MinProxies {
				scaleStatefulSet(int(config.MinProxies))
			} else if proxies.Count > config.MaxProxies {
				scaleStatefulSet(int(config.MaxProxies))
			}
		}

		proxies.CountMu.Unlock()
	}

	debugPrint(2, "[+] Updated config: %v", config)

	return nil
}

// Updates the global info regarding the StatefulSet
func updateStatefulSet(set *v1.StatefulSet) {
	// Update proxy list
	if err := updateProxyList(set); err != nil {
		log.Fatalf("[!] Failed to update proxy list: %v", err)
	}

	// Update configs
	if err := updateProxyConfig(set.GetObjectMeta().GetAnnotations()); err != nil {
		log.Fatalf("[!] Failed to update proxy config: %v", err)
	}
}

// Starts a watcher for modifications to the StatefulSet and pods
func startWatcher() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("[!] Error getting the cluster config: %s", err)
	}

	kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("[!] Error setting up KubeClient: %s", err)
	}

	// Do an initial StatefulSet query
	statefulSet, err := kubeClient.AppsV1().StatefulSets(ProxyNamespace).Get(context.Background(), ProxyStatefulSet, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("[!] Error querying initial StatefulSet: %v", err)
	}

	updateStatefulSet(statefulSet)

	// Watches the StatefulSet events
	go func() {
		for {
			setWatcher, err := kubeClient.AppsV1().StatefulSets(ProxyNamespace).Watch(context.Background(), metav1.ListOptions{
				LabelSelector: "component=" + ProxyStatefulSet,
				Watch:         true,
			})

			if err != nil {
				log.Fatalf("[!] Error setting up the watcher: %v", err)
			}

			for {
				event := <-setWatcher.ResultChan()
				if event.Type == "" {
					break
				}

				debugPrint(2, "[+] Got StatefulSet event: %v", event.Type)

				if event.Type == watch.Added || event.Type == watch.Modified {
					updateStatefulSet(event.Object.(*v1.StatefulSet))
				}
			}
		}
	}()
}

// Prints periodic stats
func printStats() {
	debugPrint(1, "[+] Name:        %s", ProxyName)
	debugPrint(1, "[+] Namespace:   %s", ProxyNamespace)
	debugPrint(1, "[+] StatefulSet: %s", ProxyStatefulSet)

	go func() {
		for {
			time.Sleep(2 * time.Second)

			debugPrint(1, "[+] P:%-3v Max:%-4v Target:%-4v Deny:%-7v Active:%v",
				proxies.Count,
				config.MaxRequests,
				int(float64(config.MaxRequests)*config.MaxLoadFactor),
				state.DenyCounter,
				state.ActiveRequests)
		}
	}()
}

func main() {
	startWatcher()
	setupIdleShutdown()

	printStats()

	startServer()
}
