// Golang HTML5 Server Side Events Example
//
// Run this code like:
//  > go run server.go
//
// Then open up your browser to http://localhost:8080
// Your browser must support HTML5 SSE, of course.

package main

import (
	"flag"
	"fmt"
	"github.com/tarm/serial"
	"html/template"
	"log"
	"net/http"
	"time"
)

// A single Broker will be created in this program. It is responsible
// for keeping a list of which clients (browsers) are currently attached
// and broadcasting events (messages) to those clients.
//
type Broker struct {

	// Create a map of clients, the keys of the map are the channels
	// over which we can push messages to attached clients.  (The values
	// are just booleans and are meaningless.)
	//
	clients map[chan string]bool

	// Channel into which new clients can be pushed
	//
	newClients chan chan string

	// Channel into which disconnected clients should be pushed
	//
	defunctClients chan chan string

	// Channel into which messages are pushed to be broadcast out
	// to attahed clients.
	//
	messages chan string
}

// This Broker method starts a new goroutine.  It handles
// the addition & removal of clients, as well as the broadcasting
// of messages out to clients that are currently attached.
//
func (b *Broker) Start() {

	// Start a goroutine
	//
	go func() {

		// Loop endlessly
		//
		for {

			// Block until we receive from one of the
			// three following channels.
			select {

			case s := <-b.newClients:

				// There is a new client attached and we
				// want to start sending them messages.
				b.clients[s] = true
				log.Println("Added new client")

			case s := <-b.defunctClients:

				// A client has dettached and we want to
				// stop sending them messages.
				delete(b.clients, s)
				close(s)

				log.Println("Removed client")

			case msg := <-b.messages:

				// There is a new message to send.  For each
				// attached client, push the new message
				// into the client's message channel.
				for s, _ := range b.clients {
					s <- msg
				}
				log.Printf("Broadcast message to %d clients", len(b.clients))
			}
		}
	}()
}

// This Broker method handles and HTTP request at the "/events/" URL.
//
func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Make sure that the writer supports flushing.
	//
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Create a new channel, over which the broker can
	// send this client messages.
	messageChan := make(chan string)

	// Add this client to the map of those that should
	// receive updates
	b.newClients <- messageChan

	// Listen to the closing of the http connection via the CloseNotifier
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		// Remove this client from the map of attached clients
		// when `EventHandler` exits.
		b.defunctClients <- messageChan
		log.Println("HTTP connection just closed.")
	}()

	// Set the headers related to event streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Don't close the connection, instead loop 10 times,
	// sending messages and flushing the response each time
	// there is a new message to send along.
	//
	// NOTE: we could loop endlessly; however, then you
	// could not easily detect clients that dettach and the
	// server would continue to send them messages long after
	// they're gone due to the "keep-alive" header.  One of
	// the nifty aspects of SSE is that clients automatically
	// reconnect when they lose their connection.
	//
	// A better way to do this is to use the CloseNotifier
	// interface that will appear in future releases of
	// Go (this is written as of 1.0.3):
	// https://code.google.com/p/go/source/detail?name=3292433291b2
	//
	for {

		// Read from our messageChan.
		msg, open := <-messageChan

		if !open {
			// If our messageChan was closed, this means that the client has
			// disconnected.
			break
		}

		// Write to the ResponseWriter, `w`.
		fmt.Fprintf(w, "data: %s\n\n", msg)

		// Flush the response.  This is only possible if
		// the repsonse supports streaming.
		f.Flush()
	}

	// Done.
	log.Println("Finished HTTP request at ", r.URL.Path)
}

// Handler for the main page, which we wire up to the
// route at "/" below in `main`.
//
func MainPageHandler(w http.ResponseWriter, r *http.Request) {

	// Did you know Golang's ServeMux matches only the
	// prefix of the request URL?  It's true.  Here we
	// insist the path is just "/".
	if r.URL.Path != "/index.html" {
		w.WriteHeader(http.StatusNotFound)
		//		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Read in the template with our SSE JavaScript code.
	//	t, err := template.ParseFiles("html5-serial-mad/index.html")

	t, err := template.ParseFiles("html5-serial-mad/" + r.URL.Path)

	if err != nil {
		log.Fatal("WTF dude, error parsing your template: " + r.URL.Path)
	}

	// Render the template, writing to `w`.
	t.Execute(w, r.RemoteAddr)

	// Done.
	log.Println("Finished HTTP request at ", r.URL.Path)
}

var (
	comPort, cmd, httpPort string
	baud                   int
	comTimeOut             time.Duration
)

func init() {
	flag.IntVar(&baud, "baud", 9600, "baud rate")
	flag.DurationVar(&comTimeOut, "serialtimeout", time.Second*10, "serial timeout (100ms, 1s, 2s...)")
	flag.StringVar(&comPort, "port", "com1", "com port")
	flag.StringVar(&cmd, "cmd", "#011", "cmd")
	flag.StringVar(&httpPort, "httpport", "8080", "http listen port")

}

// Main routine
//
func main() {

	flag.Parse()
	cmd += "\r"
	fmt.Println("com:", comPort)
	fmt.Println("baud:", baud)
	fmt.Println("serialtimeout:", comTimeOut)
	fmt.Println("httpport:", httpPort)
	fmt.Println("cmd:", cmd)

	//	fmt.Println("com:", *word1Ptr)
	//	fmt.Println("Baud:", *numb1Ptr)
	//	fmt.Println("Cmd:", *word2Ptr)
	//sample := "#011\r"
	//fmt.Println(sample)
	//fmt.Printf("% x\n", sample)
	fmt.Println("cmd slice:", []byte(cmd))

	// Make a new Broker instance
	b := &Broker{
		make(map[chan string]bool),
		make(chan (chan string)),
		make(chan (chan string)),
		make(chan string),
	}

	// Start processing events
	b.Start()

	//start cnt write-read
	ch := make(chan string)
	go func() {
		// Init Serial
		c := &serial.Config{Name: comPort, Baud: baud, ReadTimeout: comTimeOut}
		s, err := serial.OpenPort(c)
		if err != nil {
			log.Fatal("Open port: ", err)
		}
		for {
			// Write-Read from serial port
			n, err := s.Write([]byte(cmd))
			if err != nil {
				log.Fatal("Write to port: ", err)
			}

			//	time.Sleep(500 * time.Millisecond)

			buf := make([]byte, 128)
			//_, err = s.Read(buf)
			n, err = s.Read(buf)
			if err != nil {
				log.Fatal("Read from port: ", err)
			}

			// decide what was read from device
			if n == 0 {
				ch <- "no sensor value"
			}
			if n > 0 {
				ch <- string(buf[1:n])
			}
			//ch <- string(buf[:n])
			log.Printf("slice: %q len %d", buf[:n], n)
			log.Printf("string: %v len %d", string(buf[:n]), len(string(buf[:n])))
		}
	}()

	// Make b the HTTP handler for "/events/".  It can do
	// this because it has a ServeHTTP method.  That method
	// is called in a separate goroutine for each
	// request to "/events/".
	http.Handle("/events/", b)

	// Generate a constant stream of events that get pushed
	// into the Broker's messages channel and are then broadcast
	// out to any clients that are attached.
	go func() {
		for i := 0; ; i++ {

			//read response from device
			tmp1 := <-ch

			// Create a little message to send to clients,
			// including the current time.
			//b.messages <- fmt.Sprintf("%d - the time is %v - Temp - %v", i, time.Now(), buf[:n])
			//b.messages <- fmt.Sprintf("%d - %v - Temperatura - %v", i, time.Now().Format("3:04:05 PM"), tmp1)
			b.messages <- fmt.Sprintf("%d - %v - Temperatura: %v", i, time.Now().Format(time.Stamp), tmp1)
			// Print a nice log message and sleep for 5s.
			log.Printf("Sent message %d ", i)
			//time.Sleep(5 * 1e9)

		}
	}()

	//html5 boilerplate for served template files
	//	http.Handle("/css", http.StripPrefix("/css", http.FileServer(http.Dir("html5-serial-mad/css"))))
	//	http.Handle("/js", http.StripPrefix("/js", http.FileServer(http.Dir("html5-serial-mad/js"))))
	//	http.Handle("/js/vendor", http.StripPrefix("/js/vendor", http.FileServer(http.Dir("html5-serial-mad/js/vendor"))))
	//	http.Handle("/404.html", http.StripPrefix("/404.html", http.FileServer(http.Dir("html5-serial-mad/404.html"))))

	// When we get a request at "/index.html", call `MainPageHandler`
	// in a new goroutine.
	http.Handle("/", http.HandlerFunc(MainPageHandler))

	// Start the server and listen forever on port 8080.
	http.ListenAndServe(":"+httpPort, nil)
}
