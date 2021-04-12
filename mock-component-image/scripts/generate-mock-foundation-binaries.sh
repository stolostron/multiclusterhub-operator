if [ -d "./bin" ]; then
    rm -rf ./bin
fi

mkdir ./bin

GOOS=linux go build ./templates/foundation-controller.go
mv foundation-controller ./bin/controller

GOOS=linux go build ./templates/foundation-webhook.go
mv foundation-webhook ./bin/webhook

GOOS=linux go build ./templates/foundation-proxyserver.go
mv foundation-proxyserver ./bin/proxyserver