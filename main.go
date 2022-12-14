package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

func printUsageAndExit() {
	fmt.Fprintln(os.Stderr, "usage: gossh [-t] [user@]address[:port]")
	os.Exit(1)
}

func main() {
	var host, user, pwd string

	if (len(os.Args) < 2) || (len(os.Args) > 3) {
		printUsageAndExit()
	}

	vt := flag.Bool("t", false, "enables VT100 mode")
	flag.Parse()

	args := strings.Split(flag.Args()[0], "@")
	if len(args) == 1 {
		user = "glenda"
		host = args[0]
	} else if len(args) == 2 {
		user = args[0]
		host = args[1]
	}

	if strings.Index(host, ":") == -1 {
		host += ":22"
	}

	fmt.Printf("Password for %s@%s: ", user, host)
	_, err := fmt.Scanf("%s", &pwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to scan password: ", err)
		return
	}

	conf := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.Password(pwd),
		},
	}

	var conn *ssh.Client

	conn, err = ssh.Dial("tcp", host, conf)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect to the server: ", err)
		return
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to open a session: ", err)
		return
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if *vt {
		modes := ssh.TerminalModes{
			ssh.ECHO:          0,     // disable echoing
			ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
			ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
		}

		err = session.RequestPty("xterm", 24, 80, modes)
		if err != nil {
			fmt.Fprint(os.Stderr, "Failed to request pseudo terminal: ", err)
			return
		}
	}

	/* NOTE: handling stdin ourselves, so we can gracefully handle EOF */
	stdin, err := session.StdinPipe()
	if err != nil {
		fmt.Fprint(os.Stderr, "Failed to get stdin pipe: ", err)
		return
	}
	go func() {
		isEOF := false
		for !isEOF {
			/* NOTE: when in non-VT mode, we need to do a prompt ourselves */
			if !*vt {
				fmt.Print("% ")
			}

			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				if err := scanner.Err(); err == nil {
					isEOF = true
				} else {
					fmt.Fprint(os.Stderr, "Failed to scan line from stdin: ", err)
					return
				}
			}
		
			_, err := stdin.Write([]byte(scanner.Text() + "\n"))
			if err != nil {
				fmt.Fprint(os.Stderr, "Failed to write to server's stdin: ", err)
				return
			}
		}
		fmt.Println("Got EOF")
		session.Close()			
	}()

	err = session.Shell()
	if err != nil {
		fmt.Fprint(os.Stderr, "Failed to start interactive shell: ", err)
		return
	}

	session.Wait()
	fmt.Println()
}
