package annotations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Financial-Times/go-ft-http/fthttp"
	"github.com/Financial-Times/go-logger/v2"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/husobee/vestigo"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const draftsURL = "/drafts/content/:uuid/annotations"

func TestAnnotationsRWGTG(t *testing.T) {
	server := mockGtgServer(t, true)
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)

	client, err := NewAnnotationsClient(server.URL+"/%s", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	assert.NoError(t, err)

	err = client.GTG()
	assert.NoError(t, err)
}

func TestAnnotationsRWGTGFails(t *testing.T) {
	server := mockGtgServer(t, false)
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)

	client, err := NewAnnotationsClient(server.URL+"/%s", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	require.NoError(t, err)

	err = client.GTG()
	assert.EqualError(t, err, fmt.Sprintf("GTG %v returned a %v status code for generic-rw-aurora", server.URL+"/__gtg", http.StatusServiceUnavailable))
}

func TestAnnotationsRWGTGInvalidURL(t *testing.T) {
	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)
	client, err := NewAnnotationsClient(":#", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	assert.Nil(t, client, "New PublishedAnnotationsWriter should not have returned a client")
	assert.EqualError(t, err, "parse \":\": missing protocol scheme")
}

func mockGtgServer(t *testing.T, gtgOk bool) *httptest.Server {
	r := vestigo.NewRouter()
	r.Get(status.GTGPath, func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "PAC-test-annotations-publisher/Version--is-not-a-semantic-version", userAgent)

		if !gtgOk {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	return httptest.NewServer(r)
}

func mockGetAnnotations(t *testing.T, expectedTid string, annotations map[string]AnnotationsBody, documentHash string, responseStatus int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedTid, r.Header.Get(tid.TransactionIDHeader), "transaction id")

		p, _ := strconv.ParseBool(r.URL.Query().Get("sendHasBrand"))
		assert.Equal(t, p, true)

		uuid := vestigo.Param(r, "uuid")
		response, found := annotations[uuid]
		w.Header().Add(DocumentHashHeader, documentHash)

		if responseStatus != http.StatusOK {
			w.WriteHeader(responseStatus)
			msg := map[string]string{"message": "whatever"}
			json.NewEncoder(w).Encode(&msg)
		} else if found {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(&response)
		} else {
			w.WriteHeader(http.StatusNotFound)
			msg := map[string]string{"message": "not found"}
			json.NewEncoder(w).Encode(&msg)
		}
	}
}

func TestGetAnnotations(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)
	testUUID := uuid.New()
	expectedAnnotations := AnnotationsBody{Annotations: []Annotation{
		{
			Predicate: "foo",
			ConceptID: "bar",
		},
	},
	}

	testAnnotations := map[string]AnnotationsBody{
		testUUID: expectedAnnotations,
	}

	expectedHash := "hashhashhash"

	r := vestigo.NewRouter()
	r.Get(draftsURL, mockGetAnnotations(t, testTid, testAnnotations, expectedHash, http.StatusOK))

	server := httptest.NewServer(r)
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)

	client, err := NewAnnotationsClient(server.URL+"/drafts/content/%s/annotations", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	require.NoError(t, err)

	actual, actualHash, err := client.GetAnnotations(testCtx, testUUID)
	assert.NoError(t, err)
	assert.Equal(t, expectedHash, actualHash)
	assert.Equal(t, expectedAnnotations, actual)
}

func TestGetAnnotationsNotFound(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)

	r := vestigo.NewRouter()
	r.Get(draftsURL, mockGetAnnotations(t, testTid, map[string]AnnotationsBody{}, "", http.StatusNotFound))

	server := httptest.NewServer(r)
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)

	client, err := NewAnnotationsClient(server.URL+"/drafts/content/%s/annotations", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	require.NoError(t, err)

	_, _, err = client.GetAnnotations(testCtx, uuid.New())
	assert.EqualError(t, err, ErrDraftNotFound.Error())
}

func TestGetAnnotationsFailure(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)

	r := vestigo.NewRouter()
	r.Get(draftsURL, mockGetAnnotations(t, testTid, map[string]AnnotationsBody{}, "", http.StatusInternalServerError))

	server := httptest.NewServer(r)
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)

	client, err := NewAnnotationsClient(server.URL+"/drafts/content/%s/annotations", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	require.NoError(t, err)

	_, _, err = client.GetAnnotations(testCtx, uuid.New())
	assert.Contains(t, err.Error(), "returned a 500 status code")
}

func mockSaveAnnotations(t *testing.T, expectedTid string, expectedUUID string, expectedHash string, updatedDocumentHash string, expectedResponse int, respondWithBody bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedTid, r.Header.Get(tid.TransactionIDHeader), "transaction id")
		assert.Equal(t, expectedHash, r.Header.Get(PreviousDocumentHashHeader))

		uuid := vestigo.Param(r, "uuid")
		if len(expectedUUID) > 0 {
			assert.Equal(t, expectedUUID, uuid)
		}

		var body AnnotationsBody
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			msg := map[string]string{"message": "failed to deserialize body: " + err.Error()}
			json.NewEncoder(w).Encode(&msg)
			return
		}

		switch expectedResponse {
		case http.StatusOK:
			fallthrough
		case http.StatusCreated:
			w.Header().Add(DocumentHashHeader, updatedDocumentHash)
			w.WriteHeader(expectedResponse)
			if respondWithBody {
				json.NewEncoder(w).Encode(&body)
			}
		default:
			w.WriteHeader(http.StatusInternalServerError)
			msg := map[string]string{"message": "test error"}
			json.NewEncoder(w).Encode(&msg)
		}
	}
}

func TestSaveAnnotations(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)
	testUUID := uuid.New()
	testAnnotations := AnnotationsBody{
		Annotations: []Annotation{
			{
				Predicate: "foo",
				ConceptID: "bar",
			},
		},
	}

	updatedHash := "newhashnewhashnewhash"
	previousHash := "oldhasholdhasholdhash"

	r := vestigo.NewRouter()
	r.Put(draftsURL, mockSaveAnnotations(t, testTid, testUUID, previousHash, updatedHash, http.StatusOK, true))

	server := httptest.NewServer(r)
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)

	client, err := NewAnnotationsClient(server.URL+"/drafts/content/%s/annotations", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	require.NoError(t, err)

	actual, actualHash, err := client.SaveAnnotations(testCtx, testUUID, previousHash, testAnnotations)
	assert.NoError(t, err)
	assert.Equal(t, updatedHash, actualHash)
	assert.Equal(t, testAnnotations, actual)
}

func TestSaveAnnotationsCreatedStatus(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)
	testUUID := uuid.New()
	testAnnotations := AnnotationsBody{Annotations: []Annotation{
		{
			Predicate: "foo",
			ConceptID: "bar",
		},
	},
	}

	updatedHash := "newhashnewhashnewhash"
	previousHash := "oldhasholdhasholdhash"

	r := vestigo.NewRouter()
	r.Put(draftsURL, mockSaveAnnotations(t, testTid, testUUID, previousHash, updatedHash, http.StatusCreated, true))

	server := httptest.NewServer(r)
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)

	client, err := NewAnnotationsClient(server.URL+"/drafts/content/%s/annotations", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	require.NoError(t, err)

	actual, actualHash, err := client.SaveAnnotations(testCtx, testUUID, previousHash, testAnnotations)
	assert.NoError(t, err)
	assert.Equal(t, updatedHash, actualHash)
	assert.Equal(t, testAnnotations, actual)
}

func TestSaveAnnotationsError(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)
	testUUID := uuid.New()
	testAnnotations := AnnotationsBody{Annotations: []Annotation{
		{
			Predicate: "foo",
			ConceptID: "bar",
		},
	},
	}

	r := vestigo.NewRouter()
	r.Put(draftsURL, mockSaveAnnotations(t, testTid, testUUID, "", "", http.StatusInternalServerError, true))

	server := httptest.NewServer(r)
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)

	annotationsURL := server.URL + "/drafts/content/%s/annotations"
	client, err := NewAnnotationsClient(annotationsURL, testingClient, logger.NewUPPLogger("test", "DEBUG"))
	require.NoError(t, err)

	_, _, err = client.SaveAnnotations(testCtx, testUUID, "", testAnnotations)
	assert.EqualError(t, err, fmt.Sprintf("write to %s returned a 500 status code", fmt.Sprintf(annotationsURL, testUUID)))
}

func TestSaveAnnotationsWriterReturnsNoBody(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)
	testUUID := uuid.New()
	testAnnotations := AnnotationsBody{
		Annotations: []Annotation{
			{
				Predicate: "foo",
				ConceptID: "bar",
			},
		},
	}

	r := vestigo.NewRouter()
	r.Put(draftsURL, mockSaveAnnotations(t, testTid, testUUID, "", "", http.StatusOK, false))

	server := httptest.NewServer(r)
	defer server.Close()

	testingClient, err := fthttp.NewClient(
		fthttp.WithSysInfo("PAC", "test-annotations-publisher"),
	)
	require.NoError(t, err)

	client, err := NewAnnotationsClient(server.URL+"/drafts/content/%s/annotations", testingClient, logger.NewUPPLogger("test", "DEBUG"))
	require.NoError(t, err)

	actual, _, err := client.SaveAnnotations(testCtx, testUUID, "", testAnnotations)
	assert.NoError(t, err)
	assert.Equal(t, testAnnotations, actual)
}
