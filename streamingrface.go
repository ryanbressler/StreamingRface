package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"path"
	"io"
	"os"
	"net/http"
	"mime/multipart"
	"code.google.com/p/go.net/websocket"
	"strconv"
	"crypto/rand"
)

//var port *int = flag.Int("port", 23456, "Port to listen.")
var host *string = flag.String("h", "localhost:23456", "Host:Port")
var golem *string = flag.String("g", "http://glados1:8083/jobs/", "Golem master.")
var task *string = flag.String("t", "/titan/cancerregulome9/workspaces/golems/bin/run_rface_and_post.sh", "Absolute path to sh script describing task.")
var password *string = flag.String("p", "password", "Password to golem master.")

//TODO: I don't think maps are threadsafe so could break with lots of concurent inserts
var WsRegistry = map[string]chan string{}


type Task struct{
	Count int
	Args []string
}

func UniqueId() string {
        subId := make([]byte, 16)
        if _, err := rand.Read(subId); err != nil {
                fmt.Println(err)
        }
        return fmt.Sprintf("%x", subId)
}

func PostTasks(tasklist []Task){
	preader,pwriter := io.Pipe()
	
	mpf := multipart.NewWriter(pwriter)
	fmt.Println("creating request")
	r, err := http.NewRequest("POST", *golem, preader)
	if err != nil {
			fmt.Println(err)
			
	}
	r.Header.Add("content-type", "multipart/form-data; boundary="+mpf.Boundary()) 
	r.Header.Add("x-golem-apikey",*password)
	r.Header.Add("x-golem-job-label", "StreamingRface")
    r.Header.Add("x-golem-job-owner", "ryanbressler@systemsbiology.org")

	go func(){
		fmt.Println("Creating form field.")
		mpfwriter,err := mpf.CreateFormFile("jsonfile","data.json")
		if err != nil {
				fmt.Println(err)
		}
		fmt.Println("Creating json encoder.")
		jsonencoder := json.NewEncoder(mpfwriter)
		jsonencoder.Encode(tasklist)
		mpf.Close()
		pwriter.Close()
		//
		
		fmt.Println("Json encoded.")
	}()
	

	fmt.Println("Doing Request.")
	client := &http.Client{}
	resp, err := client.Do(r)
	if err != nil {
			fmt.Println(err)
			
	}
	fmt.Printf("job submision response %#v\n", resp)
	io.Copy(os.Stdout,resp.Body)
	//resp.Body.Close()
	fmt.Println("Request Done.")
	
	 
}

func SubmitTasks(ids []int,returnadd string){
	n := len(ids);
	tasklist := make([]Task,0,n)
	for i := 0; i < n; i++ {
		tasklist = append(tasklist,Task{Count:1,Args:[]string{"bash",*task,strconv.Itoa(i),returnadd}})
	}
	fmt.Printf("tasklist %#v\n", tasklist)
	fmt.Println("built task list.")
	PostTasks(tasklist)
}





func SocketStreamer(ws *websocket.Conn) {
	fmt.Printf("jsonServer %#v\n", ws.Config())
	//for {
		url:=*ws.Config().Location
		id:= UniqueId()
		url.Scheme = "http"
		url.Path="/results/"+id

		var msg []int
		
		err := websocket.JSON.Receive(ws, &msg)
		if err != nil {
			fmt.Println(err)
			//break
		}
		fmt.Printf("recv:%#v\n", msg)
		
		RestultChan:=make(chan string,0)
		WsRegistry[id]=RestultChan
		SubmitTasks(msg,url.String())

		for {
			msg:=<-RestultChan
			err = websocket.Message.Send(ws, msg)
			if err != nil {
				fmt.Println(err)
				break
			}
			fmt.Printf("send:%#v\n", msg)
		}

		/*err = websocket.JSON.Send(ws, msg)
		if err != nil {
			fmt.Println(err)
			break
		}
		fmt.Printf("send:%#v\n", msg)*/
	//}
}

func ResultHandeler(w http.ResponseWriter, req *http.Request) {
	//TODO: check to make sure it is a post with results in it
	fmt.Println("result request ",req.URL.Path)
	id := path.Base(req.URL.Path)
	val,ok := WsRegistry[id]
	if ok {
		val<-req.FormValue("results")
	}
}

func MainServer(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, `<html>
<head>
<script type="text/javascript">
var path;
var ws;

function log(msg){
	console.log(msg);
	var div = document.getElementById("logdiv");
	div.innerText = msg + "\n" + div.innerText;
}

function init() {
   log("init");
   if (ws != null) {
     ws.close();
     ws = null;
   }
 
   
   ws = new WebSocket("ws://platypus:23456/streamer");
  
   ws.onopen = function () {
      log("opened\n");
   };
   ws.onmessage = function (e) {
   	  
      log("got:" + e.data);
   };
   ws.onclose = function (e) {
     log("closed\n");
   };

};

function send() {
   
   var m = document.msgform.message.value;
   
   log("send:" + m);
   ws.send(m);
   return false;
};
</script>
<body onLoad="init();">
<form name="msgform" action="#" onsubmit="return send();">
A json array of ints coresponding to feature indexes:<input type="text" name="message" size="80" value="[1,2,3]">
<input type="submit" value="send">
</form>
<div id="logdiv"></div>
</html>
`)
}

func main() {
	flag.Parse()
	http.Handle("/streamer", websocket.Handler(SocketStreamer))
	http.HandleFunc("/results/", ResultHandeler)
	http.HandleFunc("/", MainServer)
	//fmt.Printf("http://localhost:%d/\n", *port)
	err := http.ListenAndServe(*host, nil)
	if err != nil {
		panic("ListenANdServe: " + err.Error())
	}
}