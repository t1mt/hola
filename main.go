package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	client = http.DefaultClient
)

func main() {
	p := flag.Int("p", 8888, "server port")
	flag.Parse()
	port := *p

	fmt.Printf("\n--> Server listening at: 127.0.0.1:%d\n", port)
	mux := http.NewServeMux()
	handler := NewApacheLoggingHandler(http.HandlerFunc(remote), os.Stdout)
	mux.Handle("/", handler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), mux))
}

func hola(w http.ResponseWriter, r *http.Request) {
	html := `<html>
<body>
<h2>Hola</h2>
</body>
</html>
`
	w.Write([]byte(html))
}

func remote(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	p := query.Get("p")

	if p == "" {
		hola(w, r)
		return
	}

	purl, err := url.ParseRequestURI(p)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	fmt.Println("=== Request ===")
	fmt.Printf("serverName: %s", r.Host)
	fmt.Printf("requestURI: %s", r.RequestURI)
	fmt.Printf("remoteAddr: %s", r.RemoteAddr)

	newReq := &http.Request{
		Method:     r.Method,
		Host:       purl.Host,
		URL:        purl,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     r.Header,
		Body:       r.Body,
	}

	if r.Header != nil {
		for key, value := range r.Header {
			val := ""
			if value != nil && len(value) == 1 {
				val = value[0]
			}
			newReq.Header.Set(key, val)
		}
	}

	newReq.Header.Add("x-request-id", r.Header.Get("x-request-id"))
	newReq.Header.Add("x-b3-traceid", r.Header.Get("x-b3-traceid"))
	newReq.Header.Add("x-b3-spanid", r.Header.Get("x-b3-spanid"))
	newReq.Header.Add("x-b3-parentspanid", r.Header.Get("x-b3-parentspanid"))
	newReq.Header.Add("x-b3-sampled", r.Header.Get("x-b3-sampled"))
	newReq.Header.Add("x-b3-flags", r.Header.Get("x-b3-flags"))
	newReq.Header.Add("x-ot-span-context", r.Header.Get("x-ot-span-context"))

	start := time.Now()
	resp, err := client.Do(newReq)
	latency := time.Since(start)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		fmt.Printf("error: %v", err)
		return
	}

	w.Write([]byte(fmt.Sprintf("<h1>%dms %s - Response from '%s' </h1><br>\r\n",
		latency.Milliseconds(), resp.Status, p)))
	io.Copy(w, resp.Body)

	for key, value := range newReq.Header {
		fmt.Printf("%s:%s\n", key, value)
	}

	fmt.Println("=== Remote Response ===")
	fmt.Printf("%s %s %s %dms \n", r.Method, p, resp.Status, latency.Milliseconds())
}

const (
	ApacheFormatPattern = "%s - - [%s] \"%s %d %d\" %f\n"
)

type ApacheLogRecord struct {
	http.ResponseWriter

	ip                    string
	time                  time.Time
	method, uri, protocol string
	status                int
	responseBytes         int64
	elapsedTime           time.Duration
}

func (r *ApacheLogRecord) Log(out io.Writer) {
	timeFormatted := r.time.Format("02/Jan/2006 03:04:05 -0700")
	requestLine := fmt.Sprintf("%s %s %s", r.method, r.uri, r.protocol)
	fmt.Fprintf(out, ApacheFormatPattern, r.ip, timeFormatted, requestLine, r.status, r.responseBytes,
		r.elapsedTime.Seconds())
}

func (r *ApacheLogRecord) Write(p []byte) (int, error) {
	written, err := r.ResponseWriter.Write(p)
	r.responseBytes += int64(written)
	return written, err
}

func (r *ApacheLogRecord) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

type ApacheLoggingHandler struct {
	handler http.Handler
	out     io.Writer
}

func NewApacheLoggingHandler(handler http.Handler, out io.Writer) http.Handler {
	return &ApacheLoggingHandler{
		handler: handler,
		out:     out,
	}
}

func (h *ApacheLoggingHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	clientIP := r.RemoteAddr
	if colon := strings.LastIndex(clientIP, ":"); colon != -1 {
		clientIP = clientIP[:colon]
	}

	record := &ApacheLogRecord{
		ResponseWriter: rw,
		ip:             clientIP,
		time:           time.Time{},
		method:         r.Method,
		uri:            r.RequestURI,
		protocol:       r.Proto,
		status:         http.StatusOK,
		elapsedTime:    time.Duration(0),
	}

	startTime := time.Now()
	h.handler.ServeHTTP(record, r)
	finishTime := time.Now()

	record.time = finishTime.UTC()
	record.elapsedTime = finishTime.Sub(startTime)

	record.Log(h.out)
}
