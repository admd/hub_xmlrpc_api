package server

import (
	"log"
	"net/http"
	"sync"

	"github.com/chiaradiamarcelo/hub_xmlrpc_api/client"
	"github.com/chiaradiamarcelo/hub_xmlrpc_api/config"
	"github.com/chiaradiamarcelo/hub_xmlrpc_api/session"
	"github.com/gorilla/rpc"
)

var conf = config.New()
var apiSession = session.New()

//TODO:WE SHOULD GET THIS FROM SUMA API (ie, on listUserSystems)
var serverURLByServerID = map[int64]string{1000010000: "http://192.168.122.203/rpc/api"}

func isHubSessionValid(in string) bool {
	//TODO: we should check this on session or through the SUMA api
	return in == apiSession.GetHubSessionKey()
}

func getServerURLFromServerID(serverID int64) string {
	//TODO:
	return serverURLByServerID[serverID]
}

type DefaultService struct{}

type DefaultCallArgs struct {
	HubKey     string
	ServerArgs [][]interface{}
}

func (h *DefaultService) DefaultMethod(r *http.Request, args *DefaultCallArgs, reply *struct{ Data map[string]interface{} }) error {
	if isHubSessionValid(args.HubKey) {
		method, _ := NewCodec().NewRequest(r).Method()

		serverArgsByURL := make(map[string][]interface{})

		for _, args := range args.ServerArgs {
			//TODO: support methods that don't need sessionkey
			url := apiSession.GetServerURLbyServerKey(args[0].(string))
			serverArgsByURL[url] = args
		}
		reply.Data = multicastCall(method, serverArgsByURL)
	} else {
		log.Println("Hub session invalid error")
	}
	return nil
}

func multicastCall(method string, serverArgsByURL map[string][]interface{}) map[string]interface{} {
	responses := make(map[string]interface{})
	//Execute the calls concurrently and wait until we get the response from all the servers.
	var wg sync.WaitGroup

	wg.Add(len(serverArgsByURL))

	for url, args := range serverArgsByURL {
		go func(url string, args []interface{}) {
			defer wg.Done()
			response, err := executeXMLRPCCall(url, method, args)
			if err != nil {
				log.Println("Call error: %v", err)
			}
			responses[url] = response
			log.Printf("Response: %s\n", response)
		}(url, args)
	}
	wg.Wait()
	return responses
}

func executeXMLRPCCall(url string, method string, args []interface{}) (reply interface{}, err error) {
	client, err := client.GetClientWithTimeout(url, 2, 5)
	if err != nil {
		return
	}
	defer client.Close()

	err = client.Call(method, args, &reply)

	return reply, err
}

func InitServer() {
	xmlrpcCodec := NewCodec()
	xmlrpcCodec.RegisterMethod("Auth.Login")
	xmlrpcCodec.RegisterMethod("Auth.AttachToServer")
	xmlrpcCodec.RegisterDefaultMethod("DefaultService.DefaultMethod")

	RPC := rpc.NewServer()
	RPC.RegisterCodec(xmlrpcCodec, "text/xml")
	RPC.RegisterService(new(Auth), "")
	RPC.RegisterService(new(DefaultService), "")

	http.Handle("/RPC2", RPC)

	log.Println("Starting XML-RPC server on localhost:8000/RPC2")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
