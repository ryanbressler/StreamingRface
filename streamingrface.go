package main

import (
	"code.google.com/p/go.net/websocket"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

//var port *int = flag.Int("port", 23456, "Port to listen.")
var host *string = flag.String("h", "localhost:23456", "Host:Port")
var golem *string = flag.String("g", "http://glados1:8083/jobs/", "Golem master.")
var task *string = flag.String("t", "/titan/cancerregulome9/workspaces/golems/bin/run_rface_and_post.sh", "Absolute path to sh script describing task.")
var password *string = flag.String("p", "password", "Password to golem master.")

//Global registry to keep track of where to send incoming results
//TODO: maps aren't threadsafe so could break with lots of concurent inserts and should be RW mutexed
var WsRegistry = map[string]chan string{}

//Task struct used for json serialization and submission to golem
type Task struct {
	Count int
	Args  []string
}

//generate a unique random string
func UniqueId() string {
	subId := make([]byte, 16)
	if _, err := rand.Read(subId); err != nil {
		fmt.Println(err)
	}
	return fmt.Sprintf("%x", subId)
}

// posts a []Task to golem using the global configuration variables
func GolemPostTasks(tasklist []Task) {
	preader, pwriter := io.Pipe()

	mpf := multipart.NewWriter(pwriter)
	fmt.Println("creating request")
	r, err := http.NewRequest("POST", *golem, preader)
	if err != nil {
		fmt.Println(err)

	}
	r.Header.Add("content-type", "multipart/form-data; boundary="+mpf.Boundary())
	r.Header.Add("x-golem-apikey", *password)
	r.Header.Add("x-golem-job-label", "StreamingRface")
	//TODO: this is just a nicety and we can either eliminate or make this configurable
	r.Header.Add("x-golem-job-owner", "codefor@systemsbiology.org")

	go func() {
		fmt.Println("Creating form field.")
		mpfwriter, err := mpf.CreateFormFile("jsonfile", "data.json")
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
	io.Copy(os.Stdout, resp.Body)
	//resp.Body.Close()
	fmt.Println("Request Done.")

}

//turn a []int list of  ids into a list of tasks to be run and post it using GolemPostTasks 
func SubmitTasks(ids []int, returnadd string) {
	//TODO: the bash script task can check to see if it has results for a job but maybe we should check here to see 
	//if a job is allready in progress and if the feature actually exsists.
	n := len(ids)
	tasklist := make([]Task, 0, n)
	for i := 0; i < n; i++ {
		tasklist = append(tasklist, Task{Count: 1, Args: []string{"bash", *task, strconv.Itoa(ids[i]), returnadd}})
	}
	fmt.Printf("tasklist %#v\n", tasklist)
	fmt.Println("built task list.")
	GolemPostTasks(tasklist)
}

//Handeler for websocket connectionf from client pages. Expects a []int list of feature id's as its first message 
//and will submit these tasks and then wait to stream results back
func SocketStreamer(ws *websocket.Conn) {
	fmt.Printf("jsonServer %#v\n", ws.Config())
	//for {
	url := *ws.Config().Location
	id := UniqueId()
	url.Scheme = "http"
	url.Path = "/results/" + id

	var msg []int

	err := websocket.JSON.Receive(ws, &msg)
	if err != nil {
		fmt.Println(err)
		//break
	}
	//TODO:Consider allowing multiple tasks submissions on one channel instead of only waiting for one msg
	fmt.Printf("recv:%#v\n", msg)

	RestultChan := make(chan string, 0)
	WsRegistry[id] = RestultChan
	SubmitTasks(msg, url.String())

	for {
		msg := <-RestultChan
		err = websocket.Message.Send(ws, msg)
		if err != nil {
			fmt.Println(err)
			break
		}
		fmt.Printf("send:%#v\n", msg)
	}

	//TODO: remove ws from registry and cancel outstandign jobs when connetion dies
}

//excepts posts from tasks and dispatches them to appropriate Socket Streamer
func ResultHandeler(w http.ResponseWriter, req *http.Request) {
	//TODO: check to make sure it is a post with results in it
	//and sanatize it so people can't push arbitray stuff to the client
	fmt.Println("result request ", req.URL.Path)
	id := path.Base(req.URL.Path)
	val, ok := WsRegistry[id]
	if ok {
		select {
		case val <- req.FormValue("results"):

		case <-time.After(5 * time.Second):
			fmt.Println("timed out streaming to task ", id)
		}
	}
}

//serve the html
func MainServer(w http.ResponseWriter, req *http.Request) {
	//TODO: stop hardcoding ws url and/or move this html somewhere else
	io.WriteString(w, `<html>
<head>
<title>Wedge Demo</title>
<link href="http://orzo:8000/assets/css/vq.css" type="text/css" rel="stylesheet" />
<script type="text/javascript" src="http://orzo:8000/lib/protovis/protovis.js"></script>
<script type="text/javascript" src="http://orzo:8000/lib/d3/d3.js"></script>
<script type="text/javascript" src="http://orzo:8000/lib/d3/d3.layout.js"></script>

<script src="//ajax.googleapis.com/ajax/libs/jquery/1.7.2/jquery.min.js" type="text/javascript"></script>
<script type="text/javascript" src="http://orzo:8000/lib/underscore/underscore.js"></script>
<script type="text/javascript" src="http://orzo:8000/vq.js"></script>
<script type="text/javascript" src="http://orzo:8000/vq.wedge.js"></script>

<script type="text/javascript" src="http://orzo:8000/data/GBM_cnv_track.json"></script>
<script type="text/javascript" src="http://orzo:8000/data/GBM_link_density_track.json"></script>
<script type="text/javascript" src="http://orzo:8000/data/GBM_tick_track.json"></script>
<script type="text/javascript" src="http://orzo:8000/data/GBM_network.json"></script>
<script type="text/javascript" src="http://orzo:8000/data/chromInfo.json"></script>
<script type="text/javascript" src="http://orzo:8000/data/cytoband.hg18.json"></script>


<body onLoad="init();">
<div id="wedge" style="font: 10px sans-serif;margin:10px"></div>

<form name="msgform" action="#" onsubmit="return send();">
A json array of ints coresponding to feature indexes:<input type="text" name="message" size="80" value="[0,1,2,3,4,5,6,18162,0,30798,30797,4986,4987,4988,4989]">
<input type="submit" value="send">
</form>
<div id="logdiv"></div>
<script type="text/javascript">
var path;
var ws;
var circle;



function wedge_plot(div) {
    var width=1200, height=1200;
    var	cnv_ring_height = 60;

      var tick_keys = ['METH','CNV','hsa-miR','Other'],
        color_scale = d3.scale.category10().domain(tick_keys);

    var tick_colors = function(data) {
           var tick_key = 'Other';
           tick_keys.forEach(function(key){
           if (data.label.indexOf(key) == 0)  //starts with key
           tick_key = key;
           });
       return color_scale(tick_key);
       };


    var data = {
   PLOT: {
            container: div,
            width : width,
            height: height,
            vertical_padding : 10,
            horizontal_padding: 10,
            enable_pan : false,
            enable_zoom : false,
            show_legend: true,
            legend_corner : 'ne',
            legend_radius  : 45
        },

        GENOME: {
            DATA:{
                key_order : chrom_keys,
                key_length : chrom_leng
            },
            OPTIONS: {
                radial_grid_line_width : 2,
                label_layout_style:'clock',
                label_font_style:'16px helvetica'
           }
        },

        WEDGE:[
                 {
               PLOT : {
                height : 20,
                   type : 'karyotype'
            } ,
                DATA:{
                    data_array : cytoband
                },
                OPTIONS: {

                    legend_label : 'Karyotype Bands',
                    legend_description : 'Karyotype Bands',
                    listener : function() {return null;},
                    outer_padding: 15
                }
            },
            {   PLOT : {
                height : cnv_ring_height,
                   type : 'histogram'
            } ,
                DATA:{
                    data_array : []//cnv
                },
                OPTIONS: {
                    base_value : 0,
                    legend_label : 'Mean CNV',
                    legend_description : 'Mean Copy Number Variation',
                    listener : listener,
                    outer_padding: 15
                }
            },
            {
                PLOT : {

                    height : 60,
                    type : 'scatterplot'
                },
                DATA:{
                    data_array : [],//link_density,
                    value_key : 'value'
                },
                OPTIONS: {
                    legend_label : 'Link Density',
                    legend_description : 'Density of Inferred Links',
                    min_value : 0,
                    max_value : 6,
                    base_value : 0,
                    radius : 3,
                    shape : 'circle',
                    outer_padding: 15,
                    stroke_style : null,
                    listener : listener
                }
            }
        ],

        TICKS : {
           DATA: {
                data_array:[]// ticks.map(function(tick) { return vq.utils.VisUtils.extend({label:tick.value},tick);})
            },
            OPTIONS : {
                height : 120,
                display_legend : true,
                legend_corner : 'nw',
                fill_style : tick_colors,
                label_map : [{key:{label:'METH'}, label: 'Methylation'},
                            {key:{label:'CNV'} , label: 'Copy Number Variation'},
                            {key:{label:'hsa-miR'}, label:'uRNA'},
                            {key:{label:'Other'}, label: 'Gene Expression'}]

            }
        },

        NETWORK:{
            DATA:{
                data_array : []//
            },
            OPTIONS: {
                outer_padding : 10,
                node_highlight_mode : 'isolate',
                node_fill_style : 'steelblue',
                link_stroke_style : 'red',
                link_alpha : 0.3,
                node_key : function(node) { return vq.utils.VisUtils.options_map(node)['label'];},
                node_tooltip_items :  {
                    Chr : 'chr',
                    Start : 'start',
                    End : 'end',
                    Label : function(c) { return vq.utils.VisUtils.options_map(c)['label']; }
                },
                link_tooltip_items :  {
                    'Node 1 Chr' : 'sourceNode.chr', 'Node 1 Start' : 'sourceNode.start', 'Node 1 End' : 'sourceNode.end',
                    'Node 1 Label' : function(c) { return vq.utils.VisUtils.options_map(c.sourceNode)['label'];},
                    'Node 2 Chr' : 'targetNode.chr', 'Node 2 Start' : 'targetNode.start', 'Node 2 End' : 'targetNode.end',
                    'Node 2 Label' : function(c) { return vq.utils.VisUtils.options_map(c.targetNode)['label'];}
		}
            }
        }
    };

    var listener = function(list) { return console.log("listener!"); };
    var circle_vis = new vq.CircVis();
    var dataObject ={DATATYPE : "vq.models.CircVisData", CONTENTS : data };


    circle_vis.draw(dataObject);

    return circle_vis;
}

circle = wedge_plot(document.getElementById('wedge'));

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
   	var edge_json = JSON.parse(e.data);
   	var edges = _.filter(edge_json,function(edge) { return edge.node1.chr != "" && edge.node2.chr != "";});
   	if (edges.length > 0) {
  	   	_.each(edges,function(edge){ edge.node1.chr = edge.node1.chr.slice(3); edge.node2.chr = edge.node2.chr.slice(3);});
  	   	console.log(JSON.stringify(edges));
   	circle.addEdges(edges);
   }
   	  
      
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
</html>
`)
}

//main method, parse input, and setup webserver
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
