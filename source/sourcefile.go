/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Source's Goal:
    To read endpoints from a file to be sent to external-dns

Idea:
    If we can fill the source file in any way, then we can set endpoints easily. e.g. having a client that reads hostnames and IPs from ZeroTier and writes them to a file. Then this source reads the file a set the DNS Records.
    This way is easy to create a source without modify external-dns.

Arq:
    Propossed solution: since external-dns lives inside a K8s (or K3s) Cluster, we can set a configmap that is mounted as a file in the pod. Then we just need to modify this config map to have the endpoints updated.

Usage:
    Method NewFileSource must be called using two parameters:
        base domain name (string): a base domain name (can be empty string), so if source found domain is kung and base domain name is foo, the final domain name will be kung.foo
        file name (string): the file to watch for endpoints

When you have eliminated the impossible, whatever remains, however improbable, must be the truth.
        -- Sherlock Holmes, "The Sign of Four"

Input file sample:

[
    {
        "dnsname": "dev69",
        "addresses":
            [
                "192.168.96.101"
            ]
    },
    {
        "dnsname": "JSLJitsi",
        "addresses":
            [
                "192.168.96.95",
                "192.168.96.96"
            ]
    }

]

*/

package source

import (
    "fmt"
    "os"
    "io/ioutil"
    "context"
    "encoding/json"

    "sigs.k8s.io/external-dns/endpoint"
	log "github.com/sirupsen/logrus"
)

type sourceEndpoint struct {
    DnsName  string `json:"dnsname"`
    Addresses   []string `json:"addresses"`
}

type sourceEndpoints []sourceEndpoint

type fileSource struct {
    dnsNameBase string
    fileName    string
}

func (sc *fileSource) AddEventHandler(ctx context.Context, handler func()) {
}

// NewFakeSource creates a new fakeSource with the given config.
func NewFileSource(fqdnTemplate string, sourceFileName string) (Source, error) {

    if i, err := os.Stat(sourceFileName); err == nil {
        // path/to/whatever exists
        // check whether or not is a dir
        if i.IsDir() {
            log.Errorf("%v is a directory.",sourceFileName)
            return nil, err
        }
        log.Debugf("File %v tested ok.",sourceFileName)

    } else if os.IsNotExist(err) {
        log.Errorf("File %v does not exist.",sourceFileName)
        return nil, err
    } else {
        log.Errorf("Error occurred when testing file %v : %v.",sourceFileName, err)
        return nil, err
    }

    return &fileSource{
        dnsNameBase:    fqdnTemplate,
        fileName:       sourceFileName,
    }, nil
}

// Endpoints returns endpoint objects.
func (fs *fileSource) Endpoints() ([]*endpoint.Endpoint, error) {
    log.Debugf("Func Endpoints")

    endpoints, err := fs.generateEndpoints()
    if err != nil {
        return []*endpoint.Endpoint{}, err
    }

    log.Debugf("About to return enpoints to main controller: %v", endpoints)

    return endpoints, nil
}

func (fs *fileSource) generateEndpoints() ([]*endpoint.Endpoint, error) {
    log.Debugf("Func generateEndpoints")

    f, err := fs.readFileToStruct()
    if err != nil {
        log.Debugf("readFileToStruct returned an error: %v", err)
        return []*endpoint.Endpoint{}, err
    }
    log.Debugf("Received enpoints: %v", f)

	endpoints := make([]*endpoint.Endpoint, len(f))
    log.Debugf("About to loop through read enpoints...")
    for i, v := range(f) {
        dnsname := fmt.Sprintf("%v.%v", v.DnsName, fs.dnsNameBase)
        log.Debugf("\tDomain name: %v\tAddresses:%v", dnsname, v.Addresses)
        if v.DnsName == "" || len(v.Addresses) == 0 {
            log.Debugf("\tEmpty values!")
            endpoints[i] = &endpoint.Endpoint{}
            continue
        }
        endpoints[i] = endpoint.NewEndpoint(
            dnsname,
            endpoint.RecordTypeA,
            v.Addresses...
        )

    }

    return endpoints, nil
}

func (fs *fileSource) readFileToStruct() (sourceEndpoints, error) {
    log.Debugf("Func readFileToStruct")

    log.Debugf("Opening file %v.",fs.fileName)
    jsonFile, err := os.Open(fs.fileName)
    if err != nil {
        log.Debugf("Error while opening file %v: %v",fs.fileName, err)
        return sourceEndpoints{}, err
    }
    defer jsonFile.Close()
    data, err := ioutil.ReadAll(jsonFile)
    if err != nil {
        log.Debugf("Error while opening file %v: %v",fs.fileName, err)
        return sourceEndpoints{}, err
    }
    if data == nil {
        log.Debugf("Error while opening file %v",fs.fileName)
        return sourceEndpoints{}, nil
    }

    if len(data) == 0 {
        log.Debugf("File %v is empty",fs.fileName)
        return sourceEndpoints{},nil
    }
    log.Debugf("Read data from file %v: %v",fs.fileName,data)
    var res sourceEndpoints

    err = json.Unmarshal(data, &res)
    if err != nil {
        log.Debugf("Error while unmarshaling file %v: %v",fs.fileName, err)
        return sourceEndpoints{}, err
    }
    log.Debugf("Data  unmarshaled ok: %v",res)

    return res, nil
}
