# rawhttp

A minimal http client for testing. 

No connection pool, no fixes for RFC.


Modified from [https://github.com/projectdiscovery/rawhttp](https://github.com/projectdiscovery/rawhttp)

```go
package main

import (
	"fmt"
	"github.com/zema1/rawhttp"
	"io/ioutil"
	"net/http"
	"time"
)

func main() {
	options := &rawhttp.Options{
		Timeout:                0 * time.Second,
		FollowRedirects:        false,
		MaxRedirects:           0,
		AutomaticHostHeader:    true,
		AutomaticContentLength: false,
		ForceReadAllBody:       false,
	}
	client := rawhttp.NewClient(options)
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))
}
```