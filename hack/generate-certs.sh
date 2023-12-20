#!/usr/bin/env bash

set -e

# Directory for certificates
CERTS_DIR="./certs"
mkdir -p $CERTS_DIR

# Parameters for CN (Common Name) with order of precedence: script arg > env var > default
CA_CN=${1:-${CA_CN:-"MyCA"}}
SERVER_CN=${2:-${SERVER_CN:-"localhost"}}
CLIENT_CN=${3:-${CLIENT_CN:-"localhost"}}

# File paths
CA_KEY="$CERTS_DIR/ca.key"
CA_CERT="$CERTS_DIR/ca.crt"
SERVER_KEY="$CERTS_DIR/server.key"
SERVER_CSR="$CERTS_DIR/server.csr"
SERVER_CERT="$CERTS_DIR/server.crt"
CLIENT_KEY="$CERTS_DIR/client.key"
CLIENT_CSR="$CERTS_DIR/client.csr"
CLIENT_CERT="$CERTS_DIR/client.crt"

# Function to create certificates
create_cert() {
    KEY="$1"
    CSR="$2"
    CERT="$3"
    CN="$4"

    if [ -f "$CERT" ]; then
        echo "Certificate for $CN already exists."
        return
    fi

    openssl genrsa -out $KEY 2048
    openssl req -new -key $KEY -out $CSR -subj "/CN=$CN"
    openssl x509 -req -in $CSR -CA $CA_CERT -CAkey $CA_KEY -CAcreateserial -out $CERT -days 500 -sha256
    rm $CSR
}

# Create CA if it doesn't exist
if [ ! -f "$CA_CERT" ]; then
    openssl genrsa -out $CA_KEY 2048
    openssl req -x509 -new -nodes -key $CA_KEY -sha256 -days 1024 -out $CA_CERT -subj "/CN=$CA_CN"
else
    echo "CA certificate already exists."
fi

# Create Server Cert
create_cert $SERVER_KEY $SERVER_CSR $SERVER_CERT $SERVER_CN

# Create Client Cert
create_cert $CLIENT_KEY $CLIENT_CSR $CLIENT_CERT $CLIENT_CN

echo "Certificate generation process completed."

