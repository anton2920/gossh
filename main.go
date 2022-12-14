package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh"
)

/* TODO:
 * 1. Better prompt handling.
 * 2. Infinite execution.
 * 3. Proper signal handling.
 */

func printUsageAndExit() {
	fmt.Fprintln(os.Stderr, "usage: gossh user@server")
	os.Exit(1)
}

func main() {
	var host, user, pwd string

	if len(os.Args) != 2 {
		printUsageAndExit()
	}

	args := strings.Split(os.Args[1], "@")
	if len(args) == 1 {
		user = "glenda"
		host = args[0]
	} else if len(args) == 2 {
		user = args[0]
		host = args[1]
	} else {
		printUsageAndExit()
	}
	if strings.Index(host, ":") == -1 {
		host += ":22"
	}

	fmt.Print("Password: ")
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

	var session *ssh.Session
	var stdin io.WriteCloser
	var stdout, stderr io.Reader

	session, err = conn.NewSession()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to open a session: ", err)
		return
	}
	defer session.Close()

	stdin, err = session.StdinPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to get stdin pipe: ", err)
		return
	}

	stdout, err = session.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to get stdout pipe: ", err)
		return
	}

	stderr, err = session.StderrPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to get stderr pipe: ", err)
		return
	}

	scannerFunc := func(reader io.Reader, sendChan chan<- string) {
		scanner := bufio.NewScanner(reader)

		for scanner.Scan() {
			sendChan <- scanner.Text()
		}
		if reader != stderr {
			close(sendChan)
		}
	}

	inChan := make(chan string)
	outChan := make(chan string, 128)

	go scannerFunc(os.Stdin, inChan)
	go scannerFunc(stdout, outChan)
	go scannerFunc(stderr, outChan)

	promptChan := make(chan struct{}, 1)
	session.Shell()

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGINT)

	/* NOTE: main loop */
	const prompt = "% "

	if len(promptChan) == 0 {
		promptChan <- struct{}{}
	}
mainFor:
	for {
		select {
		case signal := <-sigChan:
			switch signal {
			case syscall.SIGINT:
				session.Signal(ssh.SIGINT)
			}
		case outstr, ok := <-outChan:
			if !ok {
				break mainFor
			}

			fmt.Println(outstr)

			/* NOTE: draining output channel */
		outputFor:
			for {
				select {
				case outstr := <-outChan:
					fmt.Println(outstr)
				default:
					break outputFor
				}
			}

			if len(promptChan) == 0 {
				promptChan <- struct{}{}
			}
		case instr, ok := <-inChan:
			if !ok {
				break mainFor
			}
			_, err := stdin.Write([]byte(instr + "\n"))
			if err != nil {
				fmt.Fprintln(os.Stderr, "Failed to send a command: ", err)
				return
			}

			if len(promptChan) == 0 {
				promptChan <- struct{}{}
			}
		case <-promptChan:
			fmt.Print(prompt)
		}
	}

	fmt.Println()
}
