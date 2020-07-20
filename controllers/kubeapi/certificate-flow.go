package kubeapi

import (
	"log"
	"time"
	"os/exec"
	"strings"
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	certv1 "k8s.io/api/certificates/v1beta1"
)

func (ka *KubeApi) PerformCertificateFlow(ctx context.Context) (error) {
	csrApi := ka.kubernetes.CertificatesV1beta1().CertificateSigningRequests()
	csrName := "kube-pet-node."+ka.nodeName

	err := ka.keyStorage.EnsurePrivateKeyExists(func (keyPath string) error {
		log.Println("Generating new RSA private key at", keyPath)
		return exec.Command("openssl", "genrsa", "-out", keyPath, "2048").Run()
	})
	if err != nil {
		return err
	}

	csr, err := csrApi.Get(csrName, metav1.GetOptions{})
	if err != nil {
		if !strings.Contains(err.Error(), "not found") { // TODO
			return err
		}

		// no CSR yet, so let's make one
		csrParams := createServerAuthCsrParams(ka.nodeName, ka.nodeIP.String())
		request, err := GenerateCertRequest(ka.keyStorage, csrParams)
		if err != nil {
			return err
		}

		// submit to API
		csr, err = csrApi.Create(&certv1.CertificateSigningRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: csrName,
			},
			Spec: certv1.CertificateSigningRequestSpec{
				Request: request,
				// SignerName: "kubernetes.io/kubelet-serving",
				Usages: []certv1.KeyUsage{
					certv1.UsageDigitalSignature,
					certv1.UsageKeyEncipherment,
					certv1.UsageServerAuth,
				},
			},
		})
		if err != nil {
			return err
		}

		log.Println("Submitted request for a server auth certificate")
	}

	log.Println("Please approve my CSR!")
	log.Println("  $ kubectl certificate approve", csrName)

	for len(csr.Status.Certificate) == 0 {
		log.Println("CSR pending...")

		time.Sleep(time.Second * 5)
		csr, err = csrApi.Get(csrName, metav1.GetOptions{})
		if err != nil {
			return err
		}
	}

	for _, condition := range csr.Status.Conditions {
		log.Println("CSR", condition.Type, "-", condition.Reason, "-", condition.Message)
	}

	if err := ka.keyStorage.StoreFile(".crt", csr.Status.Certificate); err != nil {
		return err
	}
	log.Println("Write newly minted kubeapi certificate out to disk :D")

	if err := csrApi.Delete(csrName, &metav1.DeleteOptions{}); err != nil {
		log.Println("WARN: CSR cleanup failed:", err)
	}

	return nil
}
