package kubeapi

import (
	// "log"
	"io"
	"io/ioutil"
	"os/exec"

	ini "gopkg.in/ini.v1"
)

func GenerateCertRequest(keyStorage *KeyMaterialStorage, paramsSource io.WriterTo) ([]byte, error) {
	reqCmd := exec.Command("openssl", "req", "-new",
		"-key", keyStorage.GetFilePath(".key"),
		"-config", "/dev/stdin",
		"-out", "/dev/stdout")

	stdin, err := reqCmd.StdinPipe()
	if err != nil {
		return []byte{}, err
	}
	stdout, err := reqCmd.StdoutPipe()
	if err != nil {
		return []byte{}, err
	}

	// start the process
	if err := reqCmd.Start(); err != nil {
		return []byte{}, err
	}

	if _, err := paramsSource.WriteTo(stdin); err != nil {
		return []byte{}, err
	}
	stdin.Close() // so that openssl will process and exit

	request, err := ioutil.ReadAll(stdout)
	// prefer throwing Wait() error, even if ReadAll() already gave an error
	if err := reqCmd.Wait(); err != nil {
		return request, err
	}
	return request, err
}

func createServerAuthCsrParams(nodeName, nodeIP string) *ini.File {
	file := ini.Empty()

	req, _ := file.NewSection("req")
	req.NewKey("days", "30")
	req.NewKey("prompt", "no")
	req.NewKey("req_extensions", "v3_req")
	req.NewKey("distinguished_name", "dn")

	dn, _ := file.NewSection("dn")
	dn.NewKey("O", "system:nodes")
	dn.NewKey("CN", "system:node:"+nodeName)

	v3_req, _ := file.NewSection("v3_req")
	v3_req.NewKey("subjectAltName", "@alt_names")
	v3_req.NewKey("basicConstraints", "CA:FALSE")
	v3_req.NewKey("extendedKeyUsage", "serverAuth")
	v3_req.NewKey("keyUsage", "nonRepudiation, digitalSignature, keyEncipherment")

	alt_names, _ := file.NewSection("alt_names")
	alt_names.NewKey("DNS.1", nodeName)
	alt_names.NewKey("IP.1", nodeIP)

	return file
}
