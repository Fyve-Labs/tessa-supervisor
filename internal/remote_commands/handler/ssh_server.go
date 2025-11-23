package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/pkg/errors"
	gossh "golang.org/x/crypto/ssh"
)

type SSHServerConfig struct {
	UserPublicKey  string `json:"ca_public_key"`
	HostPrivateKey string `json:"host_private_key"`
}

type SSHServerHandler struct {
	UserPublicKey  gossh.PublicKey
	HostPrivateKey gossh.Signer

	listener net.Listener
	server   *ssh.Server
}

func NewSSHServerHandler(payload interface{}) (*SSHServerHandler, error) {
	req, err := JsonPayloadToConfig[SSHServerConfig](payload)
	if err != nil {
		return nil, errors.New("invalid payload type: expected SSHServerConfig")
	}

	caPublicKey, _, _, _, err := gossh.ParseAuthorizedKey([]byte(req.UserPublicKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA public key: %w", err)
	}
	private, err := gossh.ParsePrivateKey([]byte(req.HostPrivateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse host private key: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	return &SSHServerHandler{
		UserPublicKey:  caPublicKey,
		HostPrivateKey: private,
		listener:       ln,
	}, nil
}

func (h *SSHServerHandler) ListenPort() int {
	addr := h.listener.Addr().(*net.TCPAddr)
	return addr.Port
}

func (h *SSHServerHandler) Handle(ctx context.Context) error {
	slog.Info("Starting SSH server", slog.String("addr", fmt.Sprintf("127.0.0.1:%d", h.ListenPort())))

	config := &gossh.ServerConfig{
		ServerVersion: fmt.Sprintf("SSH-2.0-%s_%s", "Tessa", "TODO: version"),
	}
	config.AddHostKey(h.HostPrivateKey)

	certChecker := &UserCertChecker{
		IsUserAuthority: func(auth gossh.PublicKey) bool {
			return bytes.Equal(auth.Marshal(), h.UserPublicKey.Marshal())
		},
		certChecker: gossh.CertChecker{},
	}

	// set default handler
	ssh.Handle(h.handleSession)

	h.server = &ssh.Server{
		ServerConfigCallback: func(ctx ssh.Context) *gossh.ServerConfig {
			return config
		},
		PublicKeyHandler: func(ctx ssh.Context, pubKey ssh.PublicKey) bool {
			remoteAddr := ctx.RemoteAddr()
			permissions, err := certChecker.Authenticate(ctx.User(), pubKey)
			if err != nil {
				slog.Warn(err.Error(), "addr", remoteAddr)
				return false
			}

			ctx.SetValue(ssh.ContextKeyPermissions, &ssh.Permissions{Permissions: permissions})

			slog.Info("SSH connected", "addr", remoteAddr)
			return true
		},
		// close idle connections after 1 hour
		IdleTimeout: 3600 * time.Second,
	}

	return h.server.Serve(h.listener)
}

func (h *SSHServerHandler) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = h.listener.Close()
	h.listener = nil
	return h.server.Shutdown(ctx)
}

func (h *SSHServerHandler) handleSession(s ssh.Session) {
	cmd := exec.Command("/bin/bash")
	ptyReq, winCh, isPty := s.Pty()
	if isPty {
		cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
		f, err := pty.Start(cmd)
		if err != nil {
			panic(err)
		}

		go func() {
			for win := range winCh {
				setWinsize(f, win.Width, win.Height)
			}
		}()
		go func() {
			io.Copy(f, s) // stdin
		}()
		io.Copy(s, f) // stdout
		cmd.Wait()
	}

	s.Exit(0)
}

type UserCertChecker struct {
	IsUserAuthority func(auth gossh.PublicKey) bool
	certChecker     gossh.CertChecker
}

func (c *UserCertChecker) Authenticate(user string, pubKey gossh.PublicKey) (*gossh.Permissions, error) {
	cert, ok := pubKey.(*gossh.Certificate)
	if !ok {
		return nil, errors.New("ssh: normal key pairs not accepted")
	}

	if cert.CertType != gossh.UserCert {
		return nil, fmt.Errorf("ssh: cert has type %d", cert.CertType)
	}
	if !c.IsUserAuthority(cert.SignatureKey) {
		return nil, fmt.Errorf("ssh: certificate signed by unrecognized authority")
	}

	if err := c.certChecker.CheckCert(user, cert); err != nil {
		return nil, err
	}

	return &cert.Permissions, nil
}

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}
