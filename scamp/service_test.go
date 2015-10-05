package scamp

import "testing"
import "time"
import "bytes"
import "encoding/json"
import "net"

// TODO: fix Session API (aka, simplify design by dropping it)
// func TestServiceHandlesRequest(t *testing.T) {
// 	Initialize()

// 	hasStopped := make(chan bool, 1)
// 	service := spawnTestService(hasStopped)
// 	connectToTestService(t)
// 	service.Stop()
// 	<-hasStopped

// }

func spawnTestService(hasStopped (chan bool)) (service *Service) {
	service,err := NewService(":30100", "helloworld")
	if err != nil {
		Error.Fatalf("error creating new service: `%s`", err)
	}
	service.Register("helloworld.hello", func(req Request, sess *Session){
		if len(req.Blob) > 0 {
			Info.Printf("helloworld had data: %s", req.Blob)
		} else {
			Trace.Printf("helloworld was called without data")
		}

		err = sess.Send(Reply{
			Blob: []byte("sup"),
		})
		if err != nil {
			Error.Printf("error while sending reply: `%s`. continuing.", err)
			return
		}
		Trace.Printf("successfully responded to hello world")
	})

	go func(){
		service.Run()
		hasStopped <- true
	}()
	return
}

func connectToTestService(t *testing.T) {
	conn, err := Connect("127.0.0.1:30100")
	defer conn.Close()

	if err != nil {
		Error.Fatalf("could not connect! `%s`\n", err)
	}

	err = conn.Send(&Request{
		Action:         "helloworld.hello",
		EnvelopeFormat: ENVELOPE_JSON,
		Version:        1,
	})
	if err != nil {
		Error.Fatalf("error initiating session: `%s`", err)
		t.FailNow()
	}

	sess := conn.Recv()

	select {
		case msg := <-sess.RecvChan():
			reply,ok := msg.(Reply)
			if !ok {
				t.Errorf("expected reply")
			}
			
			if !bytes.Equal(reply.Blob, []byte("sup")) {
				t.Fatalf("did not get expected response `sup`")
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting for response")
	}

	return
}

func TestServiceToProxyMarshal(t *testing.T) {
	s := Service {
		serviceSpec: "123",
		humanName: "a-cool-name",
		name: "a-cool-name-1234",
		listenerIP: net.ParseIP("174.10.10.10"),
		listenerPort: 30100,
		actions: make(map[string]*ServiceAction),
	}
	s.Register("Logging.info", func(_ Request, _ *Session) {
	})

	serviceProxy := ServiceAsServiceProxy(&s)
	serviceProxy.timestamp = 10
	b,err := json.Marshal(&serviceProxy)
	if err != nil {
		t.Fatalf("could not serialize service proxy")
	}
	expected := []byte(`[1,"a-cool-name-1234","sector",1,5000,"beepish+tls://174.10.10.10:30100",["json"],[["Logging",["info","",1]]],10]`)
	if !bytes.Equal(b, expected) {
		t.Fatalf("expected: `%s`, got: `%s`", expected, b)
	}

}
