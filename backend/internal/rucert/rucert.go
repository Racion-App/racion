// Package rucert — сертификаты удостоверяющего центра Минцифры. Ими подписаны сайты, которых нет
// в обычных хранилищах корневых сертификатов: Росстат и API ботов MAX (platform-api2.max.ru).
// В файле корневой сертификат и промежуточный Sub CA: без промежуточного цепочка не сходится.
package rucert

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"net/http"
	"time"
)

//go:embed russian_trusted_root_ca.pem
var pem []byte

// Pool — системные корневые сертификаты и сертификаты Минцифры.
func Pool() *x509.CertPool {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	pool.AppendCertsFromPEM(pem)
	return pool
}

// Client — HTTP-клиент, который доверяет и сертификатам Минцифры.
func Client(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: Pool(), MinVersion: tls.VersionTLS12},
		},
	}
}
