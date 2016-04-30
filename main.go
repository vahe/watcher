package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/fsnotify/fsnotify"
)

var (
	watchDir    string
	recursive   bool
	ignore      string
	task        string
	waitForExit bool
	initTask    bool
)

func init() {
	flag.StringVar(&watchDir, "dir", "./", "directory to watch")
	flag.BoolVar(&recursive, "r", true, "watch directory recursively")
	flag.StringVar(&ignore, "ignore", ".git", "directory name to exclude")

	flag.StringVar(&task, "task", "", "task to run when an event occurs")
	flag.BoolVar(&waitForExit, "wait", false, "wait for task to finish running")
	flag.BoolVar(&initTask, "init", true, "starts task immidiately")
}

var (
	magenta = color.New(color.FgCyan)
	red     = color.New(color.FgRed)
	yellow  = color.New(color.FgYellow)
)

func main() {
	flag.Parse()

	var taskCmd *exec.Cmd

	if task == "" {
		log.Fatalln("Please specify a task to run. For more info see usage.")
	}

	startTask := func() *exec.Cmd {
		magenta.Println("Starting task...")
		taskCmd = exec.Command("bash", "-c", task)
		// set new process group for task; simplifies cancelling process tree
		taskCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		taskCmd.Stdin = os.Stdin
		taskCmd.Stdout = os.Stdout
		taskCmd.Stderr = os.Stdout

		err := taskCmd.Start()
		if err != nil {
			log.Fatalln(err)
		}
		return taskCmd
	}

	stopTask := func() {
		// if we need to wait for task to exit, just wait
		if waitForExit {
			magenta.Println("Waiting for task to complete.")
			err := taskCmd.Wait()
			if err != nil {
				red.Println("Wait err:", err)
			}
		} else {
			magenta.Println("Stopping task...")
			pgid, err := syscall.Getpgid(taskCmd.Process.Pid)
			if err == nil {
				syscall.Kill(-pgid, syscall.SIGTERM)
			}
			done := make(chan error, 1)
			go func() {
				done <- taskCmd.Wait()
			}()
			select {
			case <-time.After(2 * time.Second):
				// sigint failed, so KILL it
				if err := taskCmd.Process.Kill(); err != nil {
					log.Fatalln("Failed to KILL task:", err)
				}
				magenta.Println("KILLed process")
			case err := <-done:
				if err != nil {
					red.Println("Stopped. Status:", err)
				} else {
					magenta.Println("Task stopped.")
				}
			}
		}
	}

	// if initTask flag was set (set by default), start task before watchers
	if initTask {
		taskCmd = startTask()
	}

	// Since we're creating a new process group for the task, we need a way of
	// cleaning it up when the watcher exits, so we watch for signals to do so
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for _ = range sigchan {
			stopTask()
			os.Exit(0)
		}
	}()

	notifyChan := make(chan struct{})
	go Watch(watchDir, notifyChan, recursive, ignore)

	for {
		_ = <-notifyChan
		stopTask()
		taskCmd = startTask()
	}
}

func watch(root string, watcher *fsnotify.Watcher, rec bool, ignore string) error {
	if !rec {
		return watcher.Add(root)
	}
	err := filepath.Walk(root, func(path string, i os.FileInfo, err error) error {
		if i.IsDir() {
			if i.Name() == ignore {
				return filepath.SkipDir
			}
			watcher.Add(path)
		}
		return nil
	})
	return err
}

// Watch watches given root diretory and sends a notification on a WRITE event
func Watch(root string, notify chan struct{}, recursive bool, ignore string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		red.Println("Watcher:", err)
		return
	}
	defer watcher.Close()

	err = watch(root, watcher, recursive, ignore)
	if err != nil {
		red.Println("Watcher:", err)
		return
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				yellow.Println("Event:", event)
				notify <- struct{}{}
			}
		case err := <-watcher.Errors:
			red.Println("Watcher:", err)
		}
	}
}
