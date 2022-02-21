package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	proxy "github.com/btbd/proxy/client"
)

var count = 1
var times = 1
var delay = 1
var increment = 0
var max = 1
var numServices = 1
var useProxy = false

var stats = map[int]int{}
var mutex = sync.Mutex{}

var totalTime time.Duration
var minTime time.Duration
var maxTime time.Duration
var totalSuccess int
var maxActive int64
var active int64
var errors = map[string]int{}
var cmd = ""

var run = false

func load(url string) {
	proxy, _ := proxy.New("http://proxy.default.svc.cluster.local")

	// client := &http.Client{}
	wg := sync.WaitGroup{}
	wg2 := sync.WaitGroup{}

	// for round := 0; round < times; round++ {
	for round := 0; ; round++ {
		sent := 0

		if active == 0 && round >= times {
			break
		}

		if round < times {

			for num := 0; num < count; num++ {
				wg.Add(1)
				wg2.Add(1)
				sent++

				tmpURL := url
				if numServices > 1 {
					tmpURL = fmt.Sprintf(url, 1+((round*count)+num)%numServices)
				}

				go func(url string, num int) {
					// client := &http.Client{}
					netHTTPTransport := &http.Transport{
						/*
							MaxIdleConns:        2000,
							MaxIdleConnsPerHost: 2000,
							MaxConnsPerHost:     100,
							IdleConnTimeout:     90 * time.Second,
							TLSHandshakeTimeout: 10 * time.Second,
						*/
					}

					client := &http.Client{
						Transport: netHTTPTransport,
						/*
							Timeout:   30 * time.Second,
						*/
					}

					req, _ := http.NewRequest("GET", tmpURL, nil)
					req.Close = true

					id := fmt.Sprintf("%d", ((round+1)*count)+num)
					req.Header.Set("ID", id)

					wg2.Done()
					for !run {
						time.Sleep(10 * time.Millisecond)
					}
					index := 0
					var res *http.Response
					var err error

					val := atomic.AddInt64(&active, 1)
					if val > maxActive {
						maxActive = val
					}
					start := time.Now()
					if useProxy {
						res, err = proxy.Do(client, req)
					} else {
						res, err = client.Do(req)
						// res, err = http.Get(url)
					}
					end := time.Now()
					dur := end.Sub(start)
					atomic.AddInt64(&active, -1)

					errstr := ""
					if err == nil {
						index = res.StatusCode
						body, _ := ioutil.ReadAll(res.Body)
						res.Body.Close()

						if res.StatusCode/100 != 2 {
							errstr = fmt.Sprintf("%s: %s", res.Status, string(body))
						}
					} else {
						errstr = fmt.Sprintf("sock: %s", err.Error())
					}

					if errstr != "" {
						if e := strings.Index(errstr, "->"); e > 0 {
							s := e - 1
							for ; s >= 0; s-- {
								if errstr[s] >= '0' && errstr[s] <= '9' {
									continue
								} else {
									break
								}
							}
							errstr = errstr[:s+1] + "xxx" + errstr[e:]
						}
					}

					mutex.Lock()
					if errstr != "" {
						errors[errstr] = errors[errstr] + 1
					} else {
						totalTime += dur
						if minTime == 0 || dur < minTime {
							minTime = dur
						}
						if maxTime < dur {
							maxTime = dur
						}
						totalSuccess++
					}
					stats[-1] = stats[-1] + 1
					stats[index] = stats[index] + 1
					mutex.Unlock()
					wg.Done()
				}(tmpURL, num)
			}
			wg2.Wait()
			run = true
			if useProxy {
				wg.Wait()
			}
			time.Sleep(10 * time.Millisecond)
		}
		fmt.Printf("%2d - Sent: %3d  Active: %4d  Fail: %5d/%5d\n",
			round+1, sent, atomic.LoadInt64(&active), stats[-1]-totalSuccess, stats[-1])

		if cmd != "" {
			go func() {
				c := exec.Command("/bin/sh", "-c", cmd)
				buf, err := c.CombinedOutput()
				if err != nil {
					buf = []byte(err.Error())
				}
				if len(buf) != 0 {
					fmt.Printf("%s\n", strings.TrimSpace(string(buf)))
				}
			}()
		}

		if round+1 < times || active != 0 {
			time.Sleep(time.Duration(delay) * time.Second)
		}

		count += increment
		if max > 0 && count > max {
			count = max
		}
	}
	if !useProxy {
		wg.Wait()
	}
}

func main() {
	flag.IntVar(&count, "c", count, "Count of  messages")
	flag.IntVar(&count, "-count", count, "Count of messages")

	flag.IntVar(&times, "t", times, "Times to run each set")
	flag.IntVar(&times, "-times", times, "Times to run each set")

	flag.IntVar(&delay, "d", delay, "Delay between each set")
	flag.IntVar(&delay, "-delay", delay, "Delay between  each set")

	flag.IntVar(&increment, "i", increment, "Amount to add for each set")
	flag.IntVar(&increment, "-increment", increment, "Amount to add for each set")

	flag.IntVar(&max, "m", max, "Max count size")
	flag.IntVar(&max, "-max", max, "Max count size")

	flag.IntVar(&numServices, "s", numServices, "Number of services")
	flag.IntVar(&numServices, "-s", numServices, "Number of services")

	flag.BoolVar(&useProxy, "p", useProxy, "Use proxy")

	flag.StringVar(&cmd, "e", cmd, "Cmd to execute after each set")
	flag.StringVar(&cmd, "-exec", cmd, "Cmd to execute after each set")

	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [-c COUNT] [-t TIMES] [-d DELAY] [-i INCREMENT] [-m MAX_COUNT] [-p] [-e cmd] URL\n", os.Args[0])
		os.Exit(1)
	}

	if max < count {
		max = count
	}

	fmt.Printf("\n\n%d calls, %d times, %d inc, %d max \n", count, times, increment, max)

	load(flag.Arg(0))

	fmt.Printf("\n")
	total := stats[-1]
	fmt.Printf("Total: %d\n", total)
	for rc, val := range stats {
		title := ""
		if rc == -1 {
			continue
		} else if rc == 0 {
			title = "err"
		} else {
			title = fmt.Sprintf("%d", rc)
		}
		fmt.Printf("- %s: %d (%.1f%%)\n", title, val, float64(100*val)/float64(total))
	}

	fmt.Printf("Max Active: %d\n", maxActive)
	fmt.Printf("Min Time: %f\n", float64(minTime)/float64(time.Second))
	fmt.Printf("Avg Time: %f\n", (float64(totalTime)/float64(totalSuccess))/float64(time.Second))
	fmt.Printf("Max Time: %f\n", float64(maxTime)/float64(time.Second))

	if len(errors) > 0 {
		fmt.Printf("\nErrors:\n")
		for k, v := range errors {
			fmt.Printf("%d: %s\n", v, strings.TrimSpace(k))
		}
	}
	fmt.Printf("\n")
}
