PREFIX=/
INSTALL_DIR=/peloton-install
OUTPUT_DIR=/output
SRC_DIR="${SRC_DIR:-/peloton}"
PROTOC_VERSION="3.0.2"
GO_VERSION="1.7.3"
POST_INSTALL_FILE="${POST_INSTALL_FILE:-/post-install.sh}"

install_golang() {
    echo 'start installing golang '$GO_VERSION
    curl -O https://storage.googleapis.com/golang/go$GO_VERSION.linux-amd64.tar.gz
    tar -xvf go$GO_VERSION.linux-amd64.tar.gz
    mv go /usr/local
    export GOROOT=/usr/local/go
    export PATH=$PATH:$GOROOT/bin
}

install_protoc () {
    echo 'start installing protoc'
    wget https://github.com/google/protobuf/releases/download/v$PROTOC_VERSION/protoc-$PROTOC_VERSION-linux-x86_64.zip
    unzip -d protoc protoc-$PROTOC_VERSION-linux-x86_64.zip
    cp protoc/bin/protoc /usr/bin
    chmod 755 /usr/bin/protoc
    rm -r protoc
    rm protoc-$PROTOC_VERSION-linux-x86_64.zip

    # install protoc-gen-go plugin
    go get -u github.com/golang/protobuf/{proto,protoc-gen-go}
    cp $GOPATH/bin/protoc-gen-go /usr/bin
    chmod 755 /usr/bin/protoc-gen-go
}

build_peloton() {
    echo 'start building peloton'
    mkdir -p $GOPATH/src/code.uber.internal/infra/peloton
    cp -R $SRC_DIR/vendor/* $GOPATH/src
    cp -R $SRC_DIR $GOPATH/src/code.uber.internal/infra/
    cd $GOPATH/src/code.uber.internal/infra/peloton
    go version
    make
}

create_installation() {
    mkdir -p $INSTALL_DIR/{usr/bin,etc/peloton,etc/default/peloton}
    # we only want bins, configs, docs
    cp -R $GOPATH/src/code.uber.internal/infra/peloton/bin/* $INSTALL_DIR/usr/bin/
    cp -R $SRC_DIR/config/* $INSTALL_DIR/etc/peloton/
}


package() {(
    # TODO: make this only use the tags, because version should be baked in
    version="$(make version)"
    os="$(lsb_release -si)"
    codename="$(lsb_release -sc)"
    release="$(lsb_release -sr)"
    pkg="/$OUTPUT_DIR/peloton-$version-${os}-${release}-${codename}.deb"
    local opts=(
        -s dir
        -n peloton
        --version "${version}-${codename}"
        --iteration "1"
        --description
"Peloton is Uber's meta-framework for managing, scheduling and upgrading jobs on Mesos clusters.
 It has a few unique design principles that differentiates itself from other Mesos meta-frameworks"
        --url=https://code.uberinternal.com/w/repo/infra/peloton/
        --license Uber
        -a amd64
        --category misc
        --vendor "Uber Technologies"
        -m peloton-dev@uber.com
        --prefix=$PREFIX
        --force
        -t deb
        -p "$pkg"
        --after-install $POST_INSTALL_FILE
    )

    cd "$INSTALL_DIR"
    fpm "${opts[@]}" -- .
)}
