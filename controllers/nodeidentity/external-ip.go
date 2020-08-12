package nodeidentity

import (
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var client *http.Client = &http.Client{}

func FetchInternetV4Address() (string, error) {
	req, err := http.NewRequest("GET", "https://4.da.gd/ip?strip", nil)
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

func WatchInternetV4Address() (<-chan string, <-chan struct{}) {
	addrC := make(chan string, 1)
	readyC := make(chan struct{})
	go func(outC chan<- string, readyC chan<- struct{}) {
		var knownAddr string

		for {
			if addr, err := FetchInternetV4Address(); err != nil {
				log.Println("NodeIdentity WARN: Failed to fetch our IPv4 Address.", err)
			} else if addr != knownAddr {
				log.Println("NodeIdentity: Discovered our IPv4 Address as", addr)
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
