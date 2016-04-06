all:
	go build

linux:
	GOARCH=amd64 GOOS=linux go install
	GOARCH=amd64 GOOS=linux go build -o droplet-lb.linux

clean:
	go clean
