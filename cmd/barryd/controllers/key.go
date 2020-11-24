package controllers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/OnitiFR/barry/cmd/barryd/server"
	"github.com/OnitiFR/barry/common"
)

// ListKeysController list API keys
func ListKeysController(req *server.Request) {
	req.Response.Header().Set("Content-Type", "application/json")
	keys := req.App.APIKeysDB.List()

	var retData common.APIKeyListEntries
	for _, key := range keys {

		retData = append(retData, common.APIKeyListEntry{
			Comment: key.Comment,
		})
	}

	sort.Slice(retData, func(i, j int) bool {
		return retData[i].Comment < retData[j].Comment
	})

	enc := json.NewEncoder(req.Response)
	err := enc.Encode(&retData)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
	}
}

// NewKeyController creates and add a new API key to the DB
func NewKeyController(req *server.Request) {
	keyComment := req.HTTP.FormValue("comment")
	keyComment = strings.TrimSpace(keyComment)

	key, err := req.App.APIKeysDB.AddNew(keyComment)
	if err != nil {
		req.App.Log.Error(server.MsgGlob, err.Error())
		http.Error(req.Response, err.Error(), 500)
		return
	}

	req.Printf("key = %s\n", key.Key)
	req.Printf("Key '%s' created\n", key.Comment)
}
