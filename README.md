# watcher

Watches a directory and runs task when a watched file is changed.

```
go get  github.com/vahe/watcher
```

##### Simple usage
To watch current directory for changes, compile and run project when a file changes, simply run:
```
watcher -task="go install && prescious_binary"
```

For more info see usage below (or read source):
```
  -dir string
    	directory to watch (default "./")
  -ignore string
    	directory name to exclude (default ".git")
  -init
    	starts task immidiately (default true)
  -r	watch directory recursively (default true)
  -task string
    	task to run when an event occurs
  -wait
    	wait for task to finish running
```
