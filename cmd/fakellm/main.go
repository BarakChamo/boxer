// Command fakellm serves a scripted model on localhost for boxer's deterministic evals.
//
//	fakellm --listen 127.0.0.1:0 --commands 'uname -a' --log requests.jsonl
//
// It prints the URL it listens on to stdout and appends every model call to --log as JSON lines.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/BarakChamo/boxer/internal/fakellm"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:0", "address to listen on; port 0 picks a free one")
	commands := flag.String("commands", "uname -a", "shell commands to issue, separated by ';;'")
	tools := flag.String("tools", "", "tool preference, comma separated; ~prefix matches substrings (default: every known shell tool)")
	answer := flag.String("answer", "{{first_word}}", "final answer template")
	logPath := flag.String("log", "", "append every model call as a JSON line to this file")
	flag.Parse()

	sc := fakellm.Scenario{Commands: strings.Split(*commands, ";;"), Answer: *answer}
	if *tools != "" {
		sc.Tools = strings.Split(*tools, ",")
	}
	srv := fakellm.New(sc)

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("http://%s\n", ln.Addr().String())
	os.Stdout.Sync()

	var logf *os.File
	if *logPath != "" {
		if logf, err = os.OpenFile(*logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	h := srv.Handler()
	logged := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		before := len(srv.Requests())
		h.ServeHTTP(w, r)
		if logf != nil {
			for _, req := range srv.Requests()[before:] {
				b, _ := json.Marshal(req)
				fmt.Fprintln(logf, string(b))
			}
		}
	})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		ln.Close()
	}()
	_ = http.Serve(ln, logged)
}
