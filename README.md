# watcher

Watches a directory and runs command when a watched file is changed.

```
go get github.com/vahe/watcher
```

##### Simple usage
To watch current directory for changes, then compile and run project when a file changes, simply run:
```
watcher -cmd="go install && prescious_binary"
```


For more info see usage below (or read source):
```
  -cmd string
      command to run when an event occurs
  -exclude string
      directory name(s) to exclude (comma separated) (default ".git")
  -init
      execute command immidiately (does not wait for first change event) (default true)
  -r  watch directory recursively (default true)
  -wait
      wait for command to finish running
  -watch string
      directory to watch (default "./")
```
