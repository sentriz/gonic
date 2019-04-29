package handler

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"github.com/sentriz/gonic/db"
)

var mockController = Controller{
	DB: db.NewMock(),
}

func TestGetArtists(t *testing.T) {
	rr := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "", nil)
	handler := http.HandlerFunc(mockController.GetArtists)
	handler.ServeHTTP(rr, r)
	dat, _ := ioutil.ReadFile("../test_data/mock_getArtists_response")
	fmt.Println(string(dat))
	fmt.Println(rr.Body)
}
