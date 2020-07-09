package plugin

import (
	"fmt"
	"net/url"
	"regexp"
	"encoding/json"
        "github.com/kassisol/hbm/storage"
	"strings"
	"github.com/docker/go-plugins-helpers/authorization"
	"github.com/kassisol/hbm/pkg/uri"
)

type plugin struct {
	appPath       string
	skipEndpoints []*regexp.Regexp
}

func stringInRegexpSlice(s string, regexps []*regexp.Regexp) bool {
	for _, re := range regexps {
		if re.MatchString(s) {
			return true
		}
	}

	return false
}

func NewPlugin(appPath string) (*plugin, error) {
	p := plugin{
		appPath: appPath,
		skipEndpoints: []*regexp.Regexp{
			regexp.MustCompile(`^/_ping`),
			regexp.MustCompile(`^/distribution/(.+)/json`),
		},
	}

	return &p, nil
}

func (p *plugin) AuthZReq(req authorization.Request) authorization.Response {
	uriinfo, err := uri.GetURIInfo(req)
	if err != nil {
		return authorization.Response{Err: err.Error()}
	}

	if req.RequestMethod == "OPTIONS" || stringInRegexpSlice(uriinfo.Path, p.skipEndpoints) {
		return authorization.Response{Allow: true}
	}

	a, err := NewApi(&uriinfo, p.appPath)
	if err != nil {
		return authorization.Response{Err: err.Error()}
	}

	r := a.Allow(req)
	if r.Error != "" {
		return authorization.Response{Err: r.Error}
	}
	if !r.Allow {
		return authorization.Response{Msg: r.Msg["text"]}
	}

	return authorization.Response{Allow: true}
}

func (p *plugin) iscreatecontainer(req authorization.Request, u *url.URL) bool {
	if req.ResponseStatusCode != 201 {
		return false
	}
	//fmt.Println("is url:", u)
	avm := regexp.MustCompile("^/v\\d+\\.\\d+/containers/create")
	if avm.MatchString(u.Path) || u.Path == "/containers/create" {
		return true
	}

	return false
}

func (p *plugin) isremovecontainer(req authorization.Request, u *url.URL) bool {
	fmt.Println("entering isremovecontainer")
	if req.ResponseStatusCode != 204 {
		return false
	}
	//fmt.Println("is url:", u)
	avm := regexp.MustCompile("^/v\\d+\\.\\d+/containers/[^/]+")
	if avm.MatchString(u.Path) && req.RequestMethod == "DELETE" {
		fmt.Println("it is removecontainer:", u.Path)
		fmt.Println("isremovecontainer req:", req)
		return true
	}

	return false
}

func (p *plugin) setcontainerowner(cname string, req authorization.Request) error {
	username := req.User
	if username == "" {
		username = "root"
	}

        s, err := storage.NewDriver("sqlite", p.appPath)
        if err != nil {
                return err
        }

	var rjson struct {
		Id string
	}
	err = json.Unmarshal(req.ResponseBody, &rjson)
	if err != nil {
		return err
	}

	s.SetContainerOwner(username, cname, rjson.Id)

	//fmt.Println("did owner with:", username, cname, rjson.Id)

	return nil
}

func (p *plugin) removecontainerowner(cname string, req authorization.Request) error {
	username := req.User
	if username == "" {
		username = "root"
	}

        s, err := storage.NewDriver("sqlite", p.appPath)
        if err != nil {
                return err
        }

	
	u, err := url.Parse(req.RequestURI)
	if err != nil {
		return  err
	}
	
	ts := strings.Trim(u.Path, "/")
	up := strings.Split(ts, "/") // api version / type / id
	if len(up) < 3 {
		return nil
	}
	if up[1] != "containers" {
		return nil
	}
	

	fmt.Println("calling s.RemoveContainerOwner")
	s.RemoveContainerOwner(username, cname, up[2])

	//fmt.Println("did owner with:", username, cname, rjson.Id)

	return nil
}

func (p *plugin) AuthZRes(req authorization.Request) authorization.Response {
	//fmt.Println("resp uri real:", req.RequestURI)
	//fmt.Println("req body:", string(req.RequestBody))
	//fmt.Println("resp body:", string(req.ResponseBody))
	u, err := url.Parse(req.RequestURI)
	if err != nil {
		//fmt.Println("parse error:", err)
		return authorization.Response{Allow: true, Msg: err.Error()}
	}
	//fmt.Println(u)

	cname := u.Query().Get("name")
	if p.iscreatecontainer(req, u) {
		//fmt.Print("setting owner for", cname)
		err = p.setcontainerowner(cname, req)
		//fmt.Println("setcontainterowner err:", err)
	}

	if p.isremovecontainer(req, u) {
		fmt.Println("calling p.removecontainerowner")
		err = p.removecontainerowner(cname, req)
	}

	return authorization.Response{Allow: true}
}
