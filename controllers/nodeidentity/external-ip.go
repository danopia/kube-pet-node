package nodeidentity

import (
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var client *http.Client = &http.Client{}

// FetchInternetAddress hits a dagd server (such as 4.da.gd, 6.da.gd) for our Internet Protocol address
func FetchInternetAddress(hostname string) (string, error) {
	req, err := http.NewRequest("GET", "https://"+hostname+"/ip?strip", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("accept", "text/plain")
	req.Header.Set("user-agent", "kube-pet-node/0.1.0 (+https://github.com/danopia/kube-pet-node)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	return string(body), nil
}

// WatchInternetAddress polls a dagd server (such as 4.da.gd, 6.da.gd) regularly for our Internet Protocol address
func WatchInternetAddress(hostname string) (<-chan string, <-chan struct{}) {
	addrC := make(chan string, 1)
	readyC := make(chan struct{})
	go func(outC chan<- string, readyC chan<- struct{}) {
		var knownAddr string

		for {
			if addr, err := FetchInternetAddress(hostname); err != nil {
				log.Println("NodeIdentity WARN: Failed to fetch our IP Address from", hostname, ":", err)
			} else if addr != knownAddr {
				log.Println("NodeIdentity: Discovered our IP Address from", hostname, "as", addr)
				outC <- addr
				knownAddr = addr
			}

			if readyC != nil {
				close(readyC)
				readyC = nil
			}
			time.Sleep(15 * time.Minute)
		}
	}(addrC, readyC)
	return addrC, readyC
}
