package annotations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	status "github.com/Financial-Times/service-status-go/httphandlers"
	tid "github.com/Financial-Times/transactionid-utils-go"
	"github.com/husobee/vestigo"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnnotationsRWGTG(t *testing.T) {
	server := mockGtgServer(t, true)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL+"/%s", timeout)
	assert.NoError(t, err)

	err = client.GTG()
	assert.NoError(t, err)
}

func TestAnnotationsRWGTGFails(t *testing.T) {
	server := mockGtgServer(t, false)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL+"/%s", timeout)
	err = client.GTG()
	assert.EqualError(t, err, fmt.Sprintf("GTG %v returned a %v status code", server.URL+"/__gtg", http.StatusServiceUnavailable))
}

func TestAnnotationsRWGTGInvalidURL(t *testing.T) {
	client, err := NewAnnotationsClient(":#", timeout)
	assert.Nil(t, client, "New PublishedAnnotationsWriter should not have returned a client")
	assert.EqualError(t, err, "parse :: missing protocol scheme")
}

func mockGtgServer(t *testing.T, gtgOk bool) *httptest.Server {
	r := vestigo.NewRouter()
	r.Get(status.GTGPath, func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "PAC annotations-publisher", userAgent)

		if !gtgOk {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	return httptest.NewServer(r)
}

func mockGetAnnotations(t *testing.T, expectedTid string, annotations map[string][]Annotation, documentHash string, responseStatus int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedTid, r.Header.Get(tid.TransactionIDHeader), "transaction id")

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
	testUuid := uuid.New()
	expectedAnnotations := []Annotation{
		{
			Predicate: "foo",
			ConceptId: "bar",
		},
	}

	testAnnotations := map[string][]Annotation{
		testUuid: expectedAnnotations,
	}

	expectedHash := "hashhashhash"

	r := vestigo.NewRouter()
	r.Get("/drafts/content/:uuid/annotations", mockGetAnnotations(t, testTid, testAnnotations, expectedHash, http.StatusOK))

	server := httptest.NewServer(r)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL+"/drafts/content/%s/annotations", timeout)
	require.NoError(t, err)

	actual, actualHash, err := client.GetAnnotations(testCtx, testUuid)
	assert.NoError(t, err)
	assert.Equal(t, expectedHash, actualHash)
	assert.Equal(t, expectedAnnotations, actual)
}

func TestGetAnnotationsNotFound(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)

	r := vestigo.NewRouter()
	r.Get("/drafts/content/:uuid/annotations", mockGetAnnotations(t, testTid, map[string][]Annotation{}, "", http.StatusNotFound))

	server := httptest.NewServer(r)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL+"/drafts/content/%s/annotations", timeout)
	require.NoError(t, err)

	_, _, err = client.GetAnnotations(testCtx, uuid.New())
	assert.EqualError(t, err, ErrDraftNotFound.Error())
}

func TestGetAnnotationsFailure(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)

	r := vestigo.NewRouter()
	r.Get("/drafts/content/:uuid/annotations", mockGetAnnotations(t, testTid, map[string][]Annotation{}, "", http.StatusInternalServerError))

	server := httptest.NewServer(r)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL+"/drafts/content/%s/annotations", timeout)
	require.NoError(t, err)

	_, _, err = client.GetAnnotations(testCtx, uuid.New())
	assert.Contains(t, err.Error(), "returned a 500 status code")
}

func mockSaveAnnotations(t *testing.T, expectedTid string, expectedUuid string, expectedHash string, updatedDocumentHash string, expectedResponse int, respondWithBody bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedTid, r.Header.Get(tid.TransactionIDHeader), "transaction id")
		assert.Equal(t, expectedHash, r.Header.Get(PreviousDocumentHashHeader))

		uuid := vestigo.Param(r, "uuid")
		if len(expectedUuid) > 0 {
			assert.Equal(t, expectedUuid, uuid)
		}

		var body []Annotation
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			log.Error(err)
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
	testUuid := uuid.New()
	testAnnotations := []Annotation{
		{
			Predicate: "foo",
			ConceptId: "bar",
		},
	}

	updatedHash := "newhashnewhashnewhash"
	previousHash := "oldhasholdhasholdhash"

	r := vestigo.NewRouter()
	r.Put("/drafts/content/:uuid/annotations", mockSaveAnnotations(t, testTid, testUuid, previousHash, updatedHash, http.StatusOK, true))

	server := httptest.NewServer(r)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL+"/drafts/content/%s/annotations", timeout)
	require.NoError(t, err)

	actual, actualHash, err := client.SaveAnnotations(testCtx, testUuid, previousHash, testAnnotations)
	assert.NoError(t, err)
	assert.Equal(t, updatedHash, actualHash)
	assert.Equal(t, testAnnotations, actual)
}

func TestSaveAnnotationsCreatedStatus(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)
	testUuid := uuid.New()
	testAnnotations := []Annotation{
		{
			Predicate: "foo",
			ConceptId: "bar",
		},
	}

	updatedHash := "newhashnewhashnewhash"
	previousHash := "oldhasholdhasholdhash"

	r := vestigo.NewRouter()
	r.Put("/drafts/content/:uuid/annotations", mockSaveAnnotations(t, testTid, testUuid, previousHash, updatedHash, http.StatusCreated, true))

	server := httptest.NewServer(r)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL+"/drafts/content/%s/annotations", timeout)
	require.NoError(t, err)

	actual, actualHash, err := client.SaveAnnotations(testCtx, testUuid, previousHash, testAnnotations)
	assert.NoError(t, err)
	assert.Equal(t, updatedHash, actualHash)
	assert.Equal(t, testAnnotations, actual)
}

func TestSaveAnnotationsError(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)
	testUuid := uuid.New()
	testAnnotations := []Annotation{
		{
			Predicate: "foo",
			ConceptId: "bar",
		},
	}

	r := vestigo.NewRouter()
	r.Put("/drafts/content/:uuid/annotations", mockSaveAnnotations(t, testTid, testUuid, "", "", http.StatusInternalServerError, true))

	server := httptest.NewServer(r)
	defer server.Close()

	annotationsUrl := server.URL + "/drafts/content/%s/annotations"
	client, err := NewAnnotationsClient(annotationsUrl, timeout)
	require.NoError(t, err)

	_, _, err = client.SaveAnnotations(testCtx, testUuid, "", testAnnotations)
	assert.EqualError(t, err, fmt.Sprintf("Write to %s returned a 500 status code", fmt.Sprintf(annotationsUrl, testUuid)))
}

func TestSaveAnnotationsWriterReturnsNoBody(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)
	testUuid := uuid.New()
	testAnnotations := []Annotation{
		{
			Predicate: "foo",
			ConceptId: "bar",
		},
	}

	r := vestigo.NewRouter()
	r.Put("/drafts/content/:uuid/annotations", mockSaveAnnotations(t, testTid, testUuid, "", "", http.StatusOK, false))

	server := httptest.NewServer(r)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL+"/drafts/content/%s/annotations", timeout)
	require.NoError(t, err)

	actual, _, err := client.SaveAnnotations(testCtx, testUuid, "", testAnnotations)
	assert.NoError(t, err)
	assert.Equal(t, testAnnotations, actual)
}
