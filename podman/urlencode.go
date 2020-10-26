package podman

import "net/url"

// UrlEncoded encodes a string like Javascript's encodeURIComponent()
func UrlEncoded(str string) (string, error) {
	u, err := url.Parse("/a/" + str)
	if err != nil {
		return "", err
	}
	return u.String()[3:], nil
}
