package bridge

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMTLSEndpointsEstablishAndPreserveBytes(t *testing.T) {
	ca, serverCert, serverKey, clientCert, clientKey := writeTLSFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	serverSource, err := newSource(ctx, Endpoint{Mode: "listen", Network: "tcp", Bind: "127.0.0.1:0", TLS: TLSOptions{Enabled: true, CAFile: ca, CertFile: serverCert, KeyFile: serverKey, RequireClientCert: true}})
	if err != nil {
		t.Fatal(err)
	}
	defer serverSource.Close()
	addr := serverSource.(*listenSource).ln.Addr().String()
	clientSource, err := newSource(ctx, Endpoint{Mode: "connect", Network: "tcp", Address: addr, DialTimeout: time.Second, Reconnect: 10 * time.Millisecond, TLS: TLSOptions{Enabled: true, CAFile: ca, CertFile: clientCert, KeyFile: clientKey, ServerName: "localhost"}})
	if err != nil {
		t.Fatal(err)
	}
	defer clientSource.Close()
	serverCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		c, e := serverSource.Acquire(ctx)
		if e != nil {
			errCh <- e
			return
		}
		serverCh <- c
	}()
	clientConn, err := clientSource.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close()
	var serverConn net.Conn
	select {
	case serverConn = <-serverCh:
	case err := <-errCh:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	defer serverConn.Close()
	payload := []byte{0, 1, 2, 3, 0xff}
	go func() { _, _ = clientConn.Write(payload) }()
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(serverConn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("TLS changed bytes: %x != %x", got, payload)
	}
}
func TestUnixEndpointsEstablishAndPreserveBytes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "gw.sock")
	serverSource, err := newSource(ctx, Endpoint{Mode: "listen", Network: "unix", Bind: path})
	if err != nil {
		t.Fatal(err)
	}
	defer serverSource.Close()
	clientSource, err := newSource(ctx, Endpoint{Mode: "connect", Network: "unix", Address: path, DialTimeout: time.Second, Reconnect: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer clientSource.Close()
	ch := make(chan net.Conn, 1)
	go func() {
		c, e := serverSource.Acquire(ctx)
		if e == nil {
			ch <- c
		}
	}()
	client, err := clientSource.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := <-ch
	defer server.Close()
	payload := []byte("unix-raw")
	go func() { _, _ = client.Write(payload) }()
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(server, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatal("unix payload changed")
	}
}
func writeTLSFixture(t *testing.T) (string, string, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	now := time.Now()
	caKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	caTpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "RC Test CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTpl, caTpl, &caKey.PublicKey, caKey)
	caPath := filepath.Join(dir, "ca.pem")
	writePEM(t, caPath, "CERTIFICATE", caDER)
	makeLeaf := func(name string, serial int64, usage x509.ExtKeyUsage) (string, string) {
		key, _ := rsa.GenerateKey(rand.Reader, 2048)
		tpl := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		if usage == x509.ExtKeyUsageServerAuth {
			tpl.DNSNames = []string{"localhost"}
		}
		der, _ := x509.CreateCertificate(rand.Reader, tpl, caTpl, &key.PublicKey, caKey)
		cp := filepath.Join(dir, name+".crt")
		kp := filepath.Join(dir, name+".key")
		writePEM(t, cp, "CERTIFICATE", der)
		writePEM(t, kp, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key))
		return cp, kp
	}
	sc, sk := makeLeaf("server", 2, x509.ExtKeyUsageServerAuth)
	cc, ck := makeLeaf("client", 3, x509.ExtKeyUsageClientAuth)
	return caPath, sc, sk, cc, ck
}
func writePEM(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := pem.Encode(f, &pem.Block{Type: typ, Bytes: der}); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
