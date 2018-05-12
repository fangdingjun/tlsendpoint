tlsendpoint
============

a tls termination util that forward the request to backend server by sni name

support many different kind of backends, tcp, unix, tls...


example:

 client A requests with sni name a.example.com forward to 127.0.0.1:8081

 client B requests with sni name b.example.com forward to
 127.0.0.1:8082

usage

    go get github.com/fangdingjun/tlsendpoint
    cp $GOPATH/src/github.com/fangdingjun/tlsendpoint/config_example.yaml config.yaml
    vim config.yaml
    $GOPATH/bin/tlsendpoint -c config.yaml