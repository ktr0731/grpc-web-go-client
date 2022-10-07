package transport

type ConnectOptions struct {
	TlsOptions *TLSOptions
}

type TLSOptions struct {
	PemClientKey         []byte
	PemClientCertificate []byte
	RootCertificates     [][]byte
}
