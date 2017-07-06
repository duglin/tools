package main

/*

jsonMod:

This program acts as a proxy in-front of a unix-socket and allows for
you to modify the incoming JSON.

Add your twiddler func to the mapping struct.

*/

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
)

var inSock = "/var/run/incoming.sock"
var outSock = "/var/run/docker.sock"
var verbose = 0
var packetSize = 4096

func log(v int, format string, args ...interface{}) {
	if verbose < v {
		return
	}
	if v == 0 {
		fmt.Fprintf(os.Stdout, format, args...)
	} else {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

// Just copy from one connection to the other
func copyConn(id int, src, tgt *net.UnixConn) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		buf := make([]byte, packetSize)
		for {
			n, err := src.Read(buf)
			if err != nil {
				break
			}
			writeN, err := tgt.Write(buf[:n])
			if err != nil || writeN != n {
				break
			}
		}
		src.CloseRead()
		tgt.CloseWrite()
		wg.Done()
	}()
	go func() {
		buf := make([]byte, packetSize)
		for {
			n, err := tgt.Read(buf)
			if err != nil {
				break
			}
			writeN, err := src.Write(buf[:n])
			if err != nil || writeN != n {
				break
			}
		}
		tgt.CloseRead()
		src.CloseWrite()
		wg.Done()
	}()

	// Wait until both sides get EOFs
	wg.Wait()
}

// read in one line, ended by \n. If we hit maxBuffer then something is wrong
func readLine(in *net.UnixConn) ([]byte, error) {
	ch := make([]byte, 1)
	maxBuffer := 10000
	line := new(bytes.Buffer)

	i := 0
	for ; i < maxBuffer; i++ {
		count, err := in.Read(ch)
		if err != nil || count == 0 {
			if count == 0 || err == io.EOF {
				break
			}
			return nil, err
		}
		line.Write(ch)
		if ch[0] == '\n' {
			break
		}
	}
	if i == maxBuffer {
		return nil, fmt.Errorf("Buffer overflow read header")
	}

	return line.Bytes(), nil
}

type tFunc func(int, [][]byte, map[string]interface{}) ([][]byte, map[string]interface{})

func parseRequest(id int, in *net.UnixConn, firstLine []byte, twiddler tFunc) {
	log(1, "%d: Modifying the request\n", id)

	headers := [][]byte{}
	var ctHeader []byte
	var line []byte
	var err error

	if twiddler != nil {
		for {
			line, err = readLine(in)
			if err != nil || len(line) == 0 {
				// No input or an error stops us - something is wrong.
				return
			}

			// Remove any Content-Length header
			if i := strings.IndexRune(string(line), ':'); i >= 1 {
				header := string(line[:i])
				if strings.EqualFold(header, "Content-Length") {
					ctHeader = line
					continue
				}
			}

			// Until we hit a blank line (the body), buffer it then loop
			if string(line) != "\r\n" {
				headers = append(headers, line)
				continue
			}

			// We hit the body, so read it in as JSON
			dec := json.NewDecoder(in)
			body := map[string]interface{}{}
			if err := dec.Decode(&body); err != nil {
				log(0, "%d: Error reading body: %#v\n", id, err)
				in.Write([]byte(fmt.Sprintf("Error parsing body: %s\n", err)))
				return
			}

			headers, body = twiddler(id, headers, body)

			// Now generate the new Body (note: it may not have changed)
			line, err = json.Marshal(body)
			if err != nil {
				log(0, "%d: Error encoding new body: %s\n%s\n", err, body)
			}

			ctHeader = []byte(fmt.Sprintf("Content-Length: %d\r\n", len(line)))

			log(5, "%d: Len:%s\nBody: %s\n", id, ctHeader, string(line))

			break
		}
	}

	// Open the connection to outSocket
	out, err := net.DialUnix("unix", nil, &net.UnixAddr{outSock, "unix"})
	if err != nil {
		log(0, "%d: Error connecting to out socket: %v\n", id, err)
		return
	}
	defer log(1, "%d: Outgoing connection closed\n", id)
	defer out.Close()

	// Match or not, write the first line
	out.Write(firstLine)

	if twiddler != nil {
		// All done, pass new header and body to docker
		for _, header := range headers {
			out.Write(header)
		}
		out.Write([]byte(ctHeader))
		out.Write([]byte("\r\n"))
		out.Write(line)
	}

	// Become a proxy/pass-thru
	copyConn(id, in, out)
}

func processRequest(id int, conn *net.UnixConn) {
	log(1, "%d: New connection\n", id)

	defer log(1, "%d: Incoming connection closed\n", id)
	defer conn.Close()

	// Grab just the first line to see if its what we're looking for
	line, err := readLine(conn)
	if err != nil {
		log(0, "%d: Error reading header line: %v\n", id, err)
		return
	}
	log(1, "%d: Request: %s\n", id, strings.TrimSpace(string(line)))

	words := strings.Split(strings.TrimSpace(string(line)), " ")
	if len(words) < 2 {
		log(0, "%d: Error extracting verb/url from header: %s\n", id, line)
	}

	for _, mapping := range mappings {
		if mapping.verb != words[0] {
			continue
		}
		if !strings.Contains(words[1], mapping.url) {
			continue
		}

		// No func means we just reject the request
		if mapping.fn == nil {
			return
		}

		parseRequest(id, conn, line, mapping.fn)
		return
	}

	// Just act like a proxy
	parseRequest(id, conn, line, nil)
}

// Add our own Label to the "docker create" cmd
func twiddleCreate(id int, headers [][]byte, body map[string]interface{}) ([][]byte, map[string]interface{}) {
	log(1, "%d: Adding a label\n", id)

	if obj := body["Labels"]; obj != nil {
		log(3, "%d: Found some labels\n", id)
		var ok bool
		labels, ok := obj.(map[string]interface{})
		if !ok {
			log(0, "%d: Error casting label: %v\n", body["Labels"])
			return nil, nil
		}

		labels["test"] = "added me!"
		body["Labels"] = labels
	} else {
		body["Labels"] = map[string]string{"test": "inserted me!"}
	}

	return headers, body
}

var mappings = []struct {
	verb string
	url  string
	fn   tFunc
}{
	{"POST", "/containers/create", twiddleCreate},
}

func main() {
	flag.StringVar(&inSock, "in", inSock, "Path to incoming socket")
	flag.StringVar(&outSock, "out", outSock, "Path to outgoing socket")
	flag.IntVar(&verbose, "v", verbose, "Verbose/debugging level")
	flag.Parse()

	connID := 0
	os.Remove(inSock)

	listener, err := net.ListenUnix("unix", &net.UnixAddr{inSock, "unix"})
	if err != nil {
		log(0, "Can't open our listener socket(%s): %v\n", inSock, err)
		os.Exit(-1)
	}
	defer os.Remove(inSock)
	log(0, "Listening on: %s\n", inSock)
	log(0, "Sending to  : %s\n", outSock)

	for {
		conn, err := listener.AcceptUnix()
		if err != nil {
			log(0, "Error in accept: %v\n", err)
			continue
		}

		connID++
		go processRequest(connID, conn)
	}
}
