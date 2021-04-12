package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// pulled from https://pkg.go.dev/k8s.io/api/admission/v1

type UID string
type Operation string
type ExtraValue []string
type GroupVersionKind struct {
	Group   string `json:"group" protobuf:"bytes,1,opt,name=group"`
	Version string `json:"version" protobuf:"bytes,2,opt,name=version"`
	Kind    string `json:"kind" protobuf:"bytes,3,opt,name=kind"`
}

type TypeMeta struct {
	// Kind is a string value representing the REST resource this object represents.
	// Servers may infer this from the endpoint the client submits requests to.
	// Cannot be updated.
	// In CamelCase.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`

	// APIVersion defines the versioned schema of this representation of an object.
	// Servers should convert recognized schemas to the latest internal value, and
	// may reject unrecognized values.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
	// +optional
	APIVersion string `json:"apiVersion,omitempty" protobuf:"bytes,2,opt,name=apiVersion"`
}

type GroupVersionResource struct {
	Group    string `json:"group" protobuf:"bytes,1,opt,name=group"`
	Version  string `json:"version" protobuf:"bytes,2,opt,name=version"`
	Resource string `json:"resource" protobuf:"bytes,3,opt,name=resource"`
}

type UserInfo struct {
	// The name that uniquely identifies this user among all active users.
	// +optional
	Username string `json:"username,omitempty" protobuf:"bytes,1,opt,name=username"`
	// A unique value that identifies this user across time. If this user is
	// deleted and another user by the same name is added, they will have
	// different UIDs.
	// +optional
	UID string `json:"uid,omitempty" protobuf:"bytes,2,opt,name=uid"`
	// The names of groups this user is a part of.
	// +optional
	Groups []string `json:"groups,omitempty" protobuf:"bytes,3,rep,name=groups"`
	// Any additional information provided by the authenticator.
	// +optional
	Extra map[string]ExtraValue `json:"extra,omitempty" protobuf:"bytes,4,rep,name=extra"`
}

type Object interface {
}

type RawExtension struct {
	// Raw is the underlying serialization of this object.
	//
	// TODO: Determine how to detect ContentType and ContentEncoding of 'Raw' data.
	Raw []byte `json:"-" protobuf:"bytes,1,opt,name=raw"`
	// Object can hold a representation of this extension - useful for working with versioned
	// structs.
	Object Object `json:"-"`
}

type AdmissionRequest struct {
	UID UID `json:"uid" protobuf:"bytes,1,opt,name=uid"`

	Kind GroupVersionKind `json:"kind" protobuf:"bytes,2,opt,name=kind"`

	Resource GroupVersionResource `json:"resource" protobuf:"bytes,3,opt,name=resource"`

	SubResource string `json:"subResource,omitempty" protobuf:"bytes,4,opt,name=subResource"`

	RequestKind *GroupVersionKind `json:"requestKind,omitempty" protobuf:"bytes,13,opt,name=requestKind"`

	RequestResource *GroupVersionResource `json:"requestResource,omitempty" protobuf:"bytes,14,opt,name=requestResource"`

	RequestSubResource string `json:"requestSubResource,omitempty" protobuf:"bytes,15,opt,name=requestSubResource"`

	Name string `json:"name,omitempty" protobuf:"bytes,5,opt,name=name"`

	Namespace string `json:"namespace,omitempty" protobuf:"bytes,6,opt,name=namespace"`

	Operation Operation `json:"operation" protobuf:"bytes,7,opt,name=operation"`
	// UserInfo is information about the requesting user
	UserInfo UserInfo `json:"userInfo" protobuf:"bytes,8,opt,name=userInfo"`

	Object RawExtension `json:"object,omitempty" protobuf:"bytes,9,opt,name=object"`

	OldObject RawExtension `json:"oldObject,omitempty" protobuf:"bytes,10,opt,name=oldObject"`

	DryRun *bool `json:"dryRun,omitempty" protobuf:"varint,11,opt,name=dryRun"`

	Options RawExtension `json:"options,omitempty" protobuf:"bytes,12,opt,name=options"`
}

type AdmissionResponse struct {
	UID UID `json:"uid" protobuf:"bytes,1,opt,name=uid"`

	Allowed bool `json:"allowed" protobuf:"varint,2,opt,name=allowed"`
}

type AdmissionReview struct {
	TypeMeta `json:",inline"`
	// Request describes the attributes for the admission request.
	// +optional
	Request *AdmissionRequest `json:"request,omitempty" protobuf:"bytes,1,opt,name=request"`
	// Response describes the attributes for the admission response.
	// +optional
	Response *AdmissionResponse `json:"response,omitempty" protobuf:"bytes,2,opt,name=response"`
}

func Ping(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %s!", r.URL.Path[1:])
}

func main() {
	webhookRun()
	// for {
	// 	time.Sleep(5 * time.Second)
	// 	fmt.Println("Still Running...")
	// }
}

func webhookRun() error {
	http.HandleFunc("/mutating", webhookServer)

	server := &http.Server{
		Addr:      ":8000",
		TLSConfig: configWebhookTLS(),
	}
	err := server.ListenAndServeTLS("", "")
	if err != nil {
		fmt.Errorf("Listen server tls error: %+v", err)
		return err
	}

	return nil
}

func webhookServer(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// The AdmissionReview that was sent to the webhook
	requestedAdmissionReview := AdmissionReview{}

	// The AdmissionReview that will be returned
	responseAdmissionReview := AdmissionReview{
		Response: &AdmissionResponse{Allowed: true},
	}

	if err := json.Unmarshal(body, &requestedAdmissionReview); err != nil {
		fmt.Errorf("error %v", err)
		return
	}

	responseAdmissionReview.Kind = requestedAdmissionReview.Kind
	responseAdmissionReview.APIVersion = requestedAdmissionReview.APIVersion
	// Return the same UID
	responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID

	respBytes, err := json.Marshal(responseAdmissionReview)
	if err != nil {
		fmt.Errorf("error %v", err)
		return
	}
	if _, err := w.Write(respBytes); err != nil {
		fmt.Errorf("error %v", err)
		return
	}
}

func configWebhookTLS() *tls.Config {
	certFile := "/var/run/ocm-webhook/tls.crt"
	keyFile := "/var/run/ocm-webhook/tls.key"
	sCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		fmt.Errorf("error %v", err)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{sCert},
	}
}
