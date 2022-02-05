# readme

Based on Liz Rice's talk at [Docker Con 2017](https://www.youtube.com/watch?v=MHv6cWjvQjM&t=1316s).

The Go code uses Linux syscalls that are only available when building for Linux:

```bash
GOOS=linux go build main.go
```