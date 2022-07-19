package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

func main() {
	concurrencyFlag := flag.Uint("concurrency", 12, "number of concurrent requests to make")
	errorFileFlag := flag.String("error-file", "", "file where to store errors")
	timeoutSecondsFlag := flag.Uint("timeout-seconds", 5, "http client timeout")
	flag.Parse()
	if *concurrencyFlag < 1 {
		fmt.Printf("-concurrency value < 1: %d\n", *concurrencyFlag)
		os.Exit(1)
	}
	var errorFile *os.File
	if *errorFileFlag != "" {
		var err error
		errorFile, err = os.Create(*errorFileFlag)
		if err != nil {
			fmt.Printf("%s: %s\n", *concurrencyFlag, err)
			os.Exit(1)
		}
	}
	concurrency := int(*concurrencyFlag)

	var input io.Reader
	input = os.Stdin

	if flag.NArg() > 0 {
		file, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Printf("failed to open file: %s\n", err)
			os.Exit(1)
		}
		input = file
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(*timeoutSecondsFlag) * time.Second,
	}

	sc := bufio.NewScanner(input)

	urls := make(chan string, 128)
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			for raw := range urls {

				u, err := url.ParseRequestURI(raw)
				if err != nil {
					log(errorFile, "%s: invalid url: %s\n", raw, err)
					continue
				}

				resp, err := fetchURL(client, u)
				if err != nil {
					log(errorFile, "%s: %s\n", u, err)
					continue
				}

				if resp.StatusCode != http.StatusOK {
					log(errorFile, "%s: non-200 response code: %s\n", u, resp.Status)
					continue
				}

				fmt.Printf("%s: OK\n", u)
			}
			wg.Done()
		}()
	}

	for sc.Scan() {
		urls <- sc.Text()
	}
	wg.Wait()

	close(urls)

	if sc.Err() != nil {
		fmt.Printf("error: %s\n", sc.Err())
	}

}

func log(errorFile *os.File, pattern string, a ...interface{}) {
	if errorFile != nil {
		fmt.Fprintf(errorFile, pattern, a...)
	}
	// Also log to stdout
	fmt.Printf(pattern, a...)
}

func fetchURL(client *http.Client, u *url.URL) (*http.Response, error) {
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Close = true
	req.Header.Set("User-Agent", "burl/0.1")

	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	if err != nil {
		return nil, err
	}

	return resp, err
}
