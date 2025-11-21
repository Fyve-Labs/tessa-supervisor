package ssh

import (
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	"golang.org/x/crypto/ssh"
)

func handleConnection(conn net.Conn, c *sshServerController) {
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, c.config)
	if err != nil {
		log.Printf("Failed to handshake for %s: %v", conn.RemoteAddr(), err)
		// Decrement on handshake failure!
		c.wg.Done()
		return
	}

	log.Printf("New SSH connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())
	c.sm.Add(sshConn)

	var once sync.Once
	cleanup := func() {
		c.sm.Remove(sshConn)
		log.Printf("Cleaned up session for %s", sshConn.RemoteAddr())
		c.wg.Done()
	}

	// Fallback cleanup if the loop exits unexpectedly
	defer once.Do(cleanup)

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Could not accept channel: %v", err)
			continue
		}

		// Associate the channel with the session for broadcasting
		c.sm.SetChannel(sshConn, channel)

		go func(in <-chan *ssh.Request) {
			var ptmx *os.File // Declare ptmx here to be accessible by window-change

			for req := range in {
				switch req.Type {
				case "pty-req":
					termLen := req.Payload[3]
					w, h := parseDims(req.Payload[termLen+4:])

					cmd := exec.Command("/bin/bash")
					cmd.Env = append(os.Environ(), "TERM=xterm")

					var ptyErr error
					ptmx, ptyErr = pty.Start(cmd)
					if ptyErr != nil {
						log.Printf("Could not start pty: %v", ptyErr)
						req.Reply(false, nil)
						return
					}

					go func() {
						err := cmd.Wait()
						if err != nil {
							log.Printf("Command wait failed for %s: %v", sshConn.RemoteAddr(), err)
						}

						// Both the PTY and the SSH channel must be closed to
						// signal the client and release all resources.
						ptmx.Close()
						log.Printf("Session ended for %s", sshConn.RemoteAddr())

						channel.Close()
						once.Do(cleanup)
					}()

					// Set the initial window size
					setWinsize(ptmx.Fd(), w, h)

					var wg sync.WaitGroup
					wg.Add(2)

					go func() {
						defer wg.Done()
						io.Copy(channel, ptmx)

					}()
					go func() {
						defer wg.Done()
						io.Copy(ptmx, channel)
					}()

					req.Reply(true, nil)
				case "shell":
					// We only accept the default shell, which is handled by pty-req.
					req.Reply(true, nil)
				case "window-change":
					if ptmx != nil {
						w, h := parseDims(req.Payload)
						setWinsize(ptmx.Fd(), w, h)
					}
				}
			}
		}(requests)
	}
}

func parseDims(b []byte) (uint32, uint32) {
	if len(b) < 8 {
		return 0, 0
	}
	w := *(*uint32)(unsafe.Pointer(&b[0]))
	h := *(*uint32)(unsafe.Pointer(&b[4]))
	return w, h
}

func setWinsize(fd uintptr, w, h uint32) {
	ws := &struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}{
		Row: uint16(h),
		Col: uint16(w),
	}
	syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}
