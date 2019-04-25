package transport

import (
    "net/http"
    // "crypto/tls"
    // "bufio"
    "net"
    "log"
    // "context"
    "time"
    // "strings"
    "github.com/fatih/color"
    "io"
    "io/ioutil"
    "sync"
)

var servers []string = []string{
    "hkbn",
    "game",
    "hkt-a",
    "hkt-b",
    "jp",
    "us",
}
/*
var configTemplate ServerConfig = ServerConfig {
        Server: "localhost:8888",
        Method: "chacha20-ietf",
        Password: "admin",
}
*/
var configTemplate ServerConfig = ServerConfig {
        Server: ".ssjs.pro:36795",
        Method: "chacha20-ietf",
        Password: "7682240408",
}

type MultiConnTransport struct {
    Transports []*http.Transport
}

func NewMultiConnTransport() *MultiConnTransport {
    mt := MultiConnTransport{}
    for i := 0; i < len(servers); i++ {
        // t := Transport{}
        t := http.Transport{
            TLSHandshakeTimeout:   3 * time.Second,
            ResponseHeaderTimeout: 3 * time.Second,
            ExpectContinueTimeout: 3 * time.Second,
        }

        serverName := servers[i]
        sc := configTemplate
        sc.Server = serverName + sc.Server

        t.Dial = func(_, addr string) (net.Conn, error) {
            return CreateTCPConn(sc, addr)
        }
        // t.name = serverName
        mt.Transports = append(mt.Transports, &t)
    }
    return &mt
}

/*
func (mt *MultiConnTransport) fetch(ctx context.Context, req *http.Request, config ServerConfig) (*http.Response, error){
    var port string
    if (req.URL.Scheme == "https") {
        port = ":443"
    } else {
        port = ":80"
    }

    req = req.WithContext(ctx)

    conn, err := CreateTCPConn(config, req.Host + port)
    if err != nil {
        return nil, err
    }

    tlsConfig := tls.Config{}
    tlsConfig.ServerName = req.Host
    tlsConn := tls.Client(conn, &tlsConfig)
    err = tlsConn.Handshake()
    if err != nil {
        return nil, err
    }

    err = req.Write(tlsConn)
    if err != nil {
        return nil, err
    }

    var resp *http.Response
    respCh := make(chan struct{})

    go func() {
        buf := bufio.NewReader(tlsConn)
        resp, err = http.ReadResponse(buf, req)
        respCh <- struct{}{}
    }()
    <-respCh

    if (resp != nil) {
        return resp, nil
    } else {
        return nil, err
    }
}
*/

type FastBody struct {
    all int
    pipeReader *io.PipeReader
    pipeWriter *io.PipeWriter

    lk *sync.Mutex
}

func newFastBody() *FastBody {
    b := FastBody{}
    b.pipeReader, b.pipeWriter = io.Pipe()
    b.lk = &sync.Mutex{}
    return &b
}

func(b *FastBody) Read(p []byte) (int, error) {
    // n, err := io.Copy(p, b.pipeReader)
    n, err := b.pipeReader.Read(p)
    return n, err
}

func(b *FastBody) Close() error {
    // close all!
    // lk.Lock()
    // b.pipeReader.Close()

    log.Println("conn close")

    // send EOF to pipe reader, then to body reader
    b.pipeWriter.Close()
    // lk.Unlock()
    return nil
}

func(mt *MultiConnTransport) FetchMulti(req *http.Request, resultCh chan *http.Response) {
    respCh := make(chan *http.Response)
    // closed := false
    // ctx, cancel := context.WithCancel(context.Background())
    start := time.Now()

    for i := 0; i < len(mt.Transports); i++ {
        t := mt.Transports[i]

        go func(t *http.Transport) {
            // resp, err := fetch(ctx, req, c)
            // req = req.WithContext(ctx)
            resp, err := t.RoundTrip(req)
            if err != nil {
                log.Println(err)
                return
            }

            /*
            if (!closed && err == nil) {
                respCh <- resp
                closed = true
            }
            */
            // log.Println(strings.ToUpper(t.name), req.URL, color.RedString("%s", time.Now().Sub(start)))
            log.Println(req.URL, color.RedString("%s", time.Now().Sub(start)))
            respCh <- resp
        }(t)

        if (req.Method != "GET") {
            break
        }
    }

    var firstResp *http.Response
    lk := &sync.Mutex{}
    body := newFastBody()

        var bufCache [][]byte
        bufCh := make(chan []byte)
        bufChClosed := false

    forwardCh := make(chan struct{})
    eofCh := make(chan struct{}, 1)
    stopAllCh := make(chan struct{})

    var correctBodyLen int64

    // cache consumer
    go func() {
        for {
            if len(bufCache) > 0 {
                seg := bufCache[0]
                n := len(seg)

                // data write to body reader
                nw, err := body.pipeWriter.Write(seg)
                if err != nil {
                    // link write process END here
                    bufCache = nil
                    log.Println(err)
                    return
                }

                if nw < n {
                    bufCache[0] = seg[nw:]
                } else {
                    bufCache = bufCache[1:]
                }
            } else {
                select {
                case <-forwardCh:
                case <-eofCh:
                    // data write successfully finished
                    log.Println("***END***", body.all, correctBodyLen)
                    body.Close()
                    close(stopAllCh)
                }
            }
        }
    }()

    // cache producer
    go func() {
        for {
            select {
            case seg, more := <-bufCh:
                if !more {
                    // TODO: last seg?
                    eofCh <- struct{}{}
                    return
                }
                bufCache = append(bufCache, seg)

                // only notify when length 0->1
                // if not then producer would be blocked by slow consumer
                if len(bufCache) == 1 {
                    forwardCh <- struct{}{}
                }
            }
        }
    }()

    for resp := range respCh {
        lk.Lock()
        isFirst := false
        if firstResp == nil {
            isFirst = true
            respClone := *resp
            firstResp = &respClone
            firstResp.Body = body

            correctBodyLen = firstResp.ContentLength

            resultCh <- firstResp
        } else {
            if (resp.StatusCode != firstResp.StatusCode) ||
               (resp.ContentLength != -1 && resp.ContentLength != firstResp.ContentLength) {
                // drain/close body
                log.Println("close body.")
                io.Copy(ioutil.Discard, resp.Body)
                resp.Body.Close()
                continue
            }
        }
        lk.Unlock()

        // 
        go func(b io.ReadCloser, first bool) {
            // if buf is defined here
            // will be overwriten by later write
            //
            // buf := make([]byte, 1024)

            counter := 0
            var eof bool

            for {
                // TODO: not working, stop 2 did it allready
                select {
                case _, more := <-stopAllCh:
                    // assert
                    if more != false {
                        log.Fatal(more)
                    }
                    log.Println("DEBUG: stop 1..")
                    io.Copy(ioutil.Discard, b)
                    b.Close()
                    return
                default:
                    _ = 0
                }

                // TODO: buffer pool
                buf := make([]byte, 1024)
                n, err := b.Read(buf)
                if err != nil {
                    // full body has been read
                    if err == io.EOF {
                        // eofCh <- struct{}{}
                        eof = true
                    } else {
                    // conn error, exit read process
                        log.Println(err)
                        return
                    }
                }
                counter += n

                if bufChClosed {
                    // log.Println("DEBUG: stop 2..")
                    io.Copy(ioutil.Discard, b)
                    b.Close()
                    return
                }

                    body.lk.Lock()
                if counter > body.all {
                    start := n - (counter - body.all)
                    end := n
                    // log.Println(n, start, end)
                    // log.Println(string(buf[start:end]))
                    if !first {
                        // log.Println(counter, body.all, n, string(buf[start:end]))
                    }
                    bufCh <- buf[start:end]
                    body.all = counter
                }
                    body.lk.Unlock()

                if eof {
                    // body.pipeWriter.Close()
                    if !bufChClosed {
                        log.Println("eof")
                        close(bufCh)
                        bufChClosed = true
                    }
                    return
                }
            }
        }(resp.Body, isFirst)
    }
}


func (mt *MultiConnTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    ch := make(chan *http.Response)
    go mt.FetchMulti(req, ch)
    resp := <-ch
    return resp, nil
}
