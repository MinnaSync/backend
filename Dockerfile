FROM golang:1.23.3

# Setup Build Directory
RUN mkdir -p /build
WORKDIR /build
COPY . /build/

# Install Dependencies & Build
RUN go mod download
RUN mkdir -p /bin
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/app.linux

# Setup Final Image
RUN rm -r /build

# Run
CMD ["/bin/app.linux"]