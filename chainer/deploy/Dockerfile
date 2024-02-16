############################################
# Stage 1: Compile the binary in a containerized Golang environment
############################################
FROM docker.io/golang:latest as build

# Copy the source files from the host
COPY . /chainer

# Set the working directory to the same place we copied the code
WORKDIR /chainer

# Build the binary!
RUN CGO_ENABLED=0 GOOS=linux go build -a -o chainer

############################################
# Stage 2: Build the container image proper
############################################
FROM scratch

# Copy the binary from the host.
COPY --from=build /chainer/chainer .

# Tell Docker to execute this command on a `docker run`.
CMD ["/chainer"]
