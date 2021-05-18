package main

import (
	"flag"
	"fmt"
	MQTT "github.com/eclipse/paho.mqtt.golang"

	pbase "github.com/synerex/synerex_proto"

	ptransit "github.com/synerex/proto_ptransit"
	pb "github.com/synerex/synerex_api"
	nodeapi "github.com/synerex/synerex_nodeapi"
	sxutil "github.com/synerex/synerex_sxutil"

	"github.com/golang/protobuf/proto"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const MQTT_CHANNEL_BUFFER = 50 // 2021

var (
	nodesrv                     = flag.String("nodesrv", "127.0.0.1:9990", "Node ID Server")
	local                       = flag.String("local", "", "Specify local synerex server")
	mqsrv                       = flag.String("mqtt_serv", "", "MQTT Server Address host:port")
	mquser                      = flag.String("mqtt_user", "", "MQTT Username")
	mqpass                      = flag.String("mqtt_pass", "", "MQTT UserID")
	mqtopic                     = flag.String("mqtt_topic", "", "MQTT Topic")
	idlist                      []uint64
	spMap                       map[uint64]*sxutil.SupplyOpts
	mu                          sync.Mutex
	sxServerAddress             string
	lastLat, lastLon, lastAngle float64
	msgCount                    int32
	sentCount                   int32
	mclient                     MQTT.Client
)

//func messageHandler(client *MQTT.Client, msg MQTT.Message) {
//	log.Printf("TOPIC: %s\n", msg.Topic())
//	log.Printf("MSG: %s\n", msg.Payload())
//}

func subscribe(client MQTT.Client, sub chan<- MQTT.Message) bool {
	log.Printf("Subscribe mqtt")
	subToken := client.Subscribe(
		*mqtopic,
		0,
		func(client MQTT.Client, msg MQTT.Message) {
			msgCount++
			sub <- msg
		})
	if subToken.Wait() && subToken.Error() != nil {
		log.Println("MQTT Subscribe error:", subToken.Error())
		return false
		//        os.Exit(1) // todo: should handle MQTT error
	}
	log.Printf("Subscribe MQTT Topic [%s] Done", *mqtopic)
	return true
}

// Converting  Docor MQTT format into PTService option.
func convertDocor2PTService(msg *string) (service *ptransit.PTService, argJson string) {
	// using docor info
	payloads := strings.Split(*msg, ",")

	log.Printf("Split into %d tokens", len(payloads))

	if payloads[0] != "DR05" {
		log.Printf("Not Docor System!")
		return service, "err"
	}else if len(payloads) > 14 {
		// for new log
		vid, _ := strconv.Atoi(payloads[1][4:]) // scrape from "KOTAXX"
		lat, err := strconv.ParseFloat(payloads[8], 64)
		if err != nil {
			log.Printf("Can't convert latitude from `%s` at %d", payloads[8], vid)
			log.Printf("ErrString:[%s]", *msg)

			// we need lat,lon to keep the map system working.
			// just use last lat, lon
			lat = lastLat // use last value
		} else {
			lastLat = lat
		}
		lon, err2 := strconv.ParseFloat(payloads[9], 64)
		if err2 != nil {
			log.Printf("Can't convert longitude from `%s` at %d", payloads[9], vid)
			log.Printf("ErrString:[%s]", *msg)
			lon = lastLon // use last value
		} else {
			lastLon = lon
		}
		angle, err3 := strconv.ParseFloat(payloads[12], 32)
		speed, _ := strconv.ParseFloat(payloads[13], 32)
		if err3 != nil {
			angle = lastAngle
			speed = 0
		} else {
			lastAngle = angle
		}
		argJson = *msg

		service = &ptransit.PTService{
			VehicleId:   int32(vid),
			Angle:       float32(angle),
			Speed:       int32(speed),
			Lat:         float32(lat),
			Lon:         float32(lon),
			VehicleType: 3, // bus number = 3 (for GTFS.Route.Type)
		}

		//      log.Printf("msg:%v",*service)
		return service, argJson

	} else { // should not come here
		log.Printf("Old MQTT format!? %s", *msg)
		argJson = *msg
		if len(payloads) > 1 {
			if len(payloads[1]) > 4 {
				vid, _ := strconv.Atoi(payloads[1][4:]) // scrape from "KOTAXX"

				service = &ptransit.PTService{
					VehicleId:   int32(vid),
					Angle:       float32(lastAngle),
					Speed:       int32(0),
					Lat:         float32(lastLat),
					Lon:         float32(lastLon),
					VehicleType: 3, // bus number = 3 (for GTFS.Route.Type)
				}
			}
		}

		return service, argJson

	}
}

var ct = 0

func handleMQMessage(sclient *sxutil.SXServiceClient, msg *string) {

	if ct < 10 || ct%100 == 0 {
		log.Printf("%d:MQTT %s\n", ct, *msg)
		ct++
		if ct < 0 {
			ct = 0
		}
	}

	pts, argJson := convertDocor2PTService(msg)

	out, err := proto.Marshal(pts)
	if err == nil {
		cont := pb.Content{Entity: out}
		smo := sxutil.SupplyOpts{
			Name:  "Docor",
			Cdata: &cont,
			JSON:  argJson,
		}
		_, nerr := sclient.NotifySupply(&smo)
		if nerr != nil {
			log.Println("Error on NotifySupply: ", nerr)

			newClient := sxutil.GrpcConnectServer(sxServerAddress)
			if newClient != nil {
				log.Printf("Reconnect Server %s\n", sxServerAddress)
				sclient.SXClient = newClient
			}
		}
	} else {
		log.Printf("Error on marshaling. %#v\ndata:[%s]", err, argJson)
	}
}

// MQTT error reconnect
func connectionLostHandler(clt MQTT.Client, err error) {
	log.Printf("MQTT Connectionlost! %v", err)

	// retry loop
	log.Printf("Connection Status con:%t  open:%t", clt.IsConnected(), clt.IsConnectionOpen())

	// may mqtt client makes reconnect

}

func mqttNewConnection(sub chan<- MQTT.Message) {
	mopts := MQTT.NewClientOptions()
	mopts.AddBroker(*mqsrv)
	mopts.SetClientID("docor-provider")
	mopts.SetUsername(*mquser)
	mopts.SetPassword(*mqpass)
	mopts.SetAutoReconnect(true)
	//    mopts.SetConnectRetry(true) // only for initial connection..

	mopts.OnConnect = func(c MQTT.Client) {
		log.Printf("MQTT Connected.")
		if !subscribe(c, sub) {
			log.Printf("Can't subscribe!")
		}
	}

	mopts.SetConnectionLostHandler(connectionLostHandler)

	mclient = MQTT.NewClient(mopts)
	token := mclient.Connect()
	token.Wait()
	merr := token.Error()
	if merr != nil {
		log.Printf("MQTT Connection Error %v", merr)
		panic(merr)
	}

}

// just for status
func monitorStatus() {
	for {
		time.Sleep(time.Second * 10)
		argStr := fmt.Sprintf("connection:%t,sent:%d", mclient.IsConnected(), sentCount)
		sxutil.SetNodeStatus(int32(msgCount), argStr)
	}
}

func main() {
	flag.Parse()
	msgCount = 0
	sentCount = 0

	go sxutil.HandleSigInt()
	sxutil.RegisterDeferFunction(sxutil.UnRegisterNode)

	channelTypes := []uint32{pbase.PT_SERVICE}
	sxo := &sxutil.SxServerOpt{
		NodeType:   nodeapi.NodeType_PROVIDER,
		ServerInfo: "docor",
		ClusterId:  0,
		AreaId:     "Default",
	}

	srv, err := sxutil.RegisterNode(*nodesrv, "DocorProvider", channelTypes, sxo)
	if err != nil {
		log.Fatal("Can't register node..", err)
	}

	if *local != "" {
		sxServerAddress = *local
		log.Printf("Connecting local %s for SXServer [%s]", sxServerAddress, srv)
	} else {
		sxServerAddress = srv
		log.Printf("Connecting SXServer [%s]", srv)
	}

	client := sxutil.GrpcConnectServer(sxServerAddress)
	log.Printf("gRPC Connected srv:%s[%v]", sxServerAddress, client)

	argJson := fmt.Sprintf("{Client:Docor}")
	sclient := sxutil.NewSXServiceClient(client, pbase.PT_SERVICE, argJson)

	log.Printf("Connected [%v]", sclient)

	// MQTT

	sub := make(chan MQTT.Message, MQTT_CHANNEL_BUFFER)

	if len(*mqsrv) == 0 {
		log.Printf("Should speficy server addresses")
		os.Exit(1)
	}

	mqttNewConnection(sub)

	//        log.Panic("Can't estabilish MQTT Connection")

	go monitorStatus()

	for {
		select {
		case s := <-sub:
			msg := string(s.Payload())
			//                              fmt.Printf("\nmsg: %s\n", msg)
			sentCount++
			handleMQMessage(sclient, &msg)
		}
	}

	sxutil.CallDeferFunctions() // cleanup!

}
