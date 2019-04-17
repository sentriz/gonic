#### GORM backend for gorilla sessions

    go get github.com/wader/gormstore

#### Documentation

http://www.godoc.org/github.com/wader/gormstore

#### Example

```go
// initialize and setup cleanup
store := gormstore.New(gorm.Open(...), []byte("secret"))
// db cleanup every hour
// close quit channel to stop cleanup
quit := make(chan struct{})
go store.PeriodicCleanup(1*time.Hour, quit)
```

```go
// in HTTP handler
func handlerFunc(w http.ResponseWriter, r *http.Request) {
  session, err := store.Get(r, "session")
  session.Values["user_id"] = 123
  store.Save(r, w, session)
  http.Error(w, "", http.StatusOK)
}
```

For more details see [gormstore godoc documentation](http://www.godoc.org/github.com/wader/gormstore).

#### Testing

Just sqlite3 tests:

    go test

All databases using docker:

    ./test

If docker is not local (docker-machine etc):

    DOCKER_IP=$(docker-machine ip dev) ./test

#### License

gormstore is licensed under the MIT license. See [LICENSE](LICENSE) for the full license text.
