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

	client, err := NewAnnotationsClient(server.URL + "/%s")
	assert.NoError(t, err)

	err = client.GTG()
	assert.NoError(t, err)
}

func TestAnnotationsRWGTGFails(t *testing.T) {
	server := mockGtgServer(t, false)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL + "/%s")
	err = client.GTG()
	assert.EqualError(t, err, fmt.Sprintf("GTG %v returned a %v status code", server.URL+"/__gtg", http.StatusServiceUnavailable))
}

func TestAnnotationsRWGTGInvalidURL(t *testing.T) {
	client, err := NewAnnotationsClient(":#")
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

func mockGetAnnotations(t *testing.T, expectedTid string, annotations map[string][]Annotation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedTid, r.Header.Get(tid.TransactionIDHeader), "transaction id")

		uuid := vestigo.Param(r, "uuid")
		if response, found := annotations[uuid]; found {
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
	r := vestigo.NewRouter()
	r.Get("/drafts/content/:uuid/annotations", mockGetAnnotations(t, testTid, testAnnotations))

	server := httptest.NewServer(r)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL + "/drafts/content/%s/annotations")
	require.NoError(t, err)

	actual, err := client.GetAnnotations(testCtx, testUuid)
	assert.NoError(t, err)

	assert.Equal(t, expectedAnnotations, actual)
}

func TestGetAnnotationsNotFound(t *testing.T) {
	testTid := "tid_test"
	testCtx := tid.TransactionAwareContext(context.Background(), testTid)
	expectedAnnotations := []Annotation{
		{
			Predicate: "foo",
			ConceptId: "bar",
		},
	}

	testAnnotations := map[string][]Annotation{
		uuid.New(): expectedAnnotations,
	}

	r := vestigo.NewRouter()
	r.Get("/drafts/content/:uuid/annotations", mockGetAnnotations(t, testTid, testAnnotations))

	server := httptest.NewServer(r)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL + "/drafts/content/%s/annotations")
	require.NoError(t, err)

	_, err = client.GetAnnotations(testCtx, uuid.New())
	assert.EqualError(t, err, ErrDraftNotFound.Error())
}

func mockSaveAnnotations(t *testing.T, expectedTid string, expectedUuid string, expectedResponse int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedTid, r.Header.Get(tid.TransactionIDHeader), "transaction id")

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
			w.WriteHeader(expectedResponse)
			json.NewEncoder(w).Encode(&body)
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

	r := vestigo.NewRouter()
	r.Put("/drafts/content/:uuid/annotations", mockSaveAnnotations(t, testTid, testUuid, http.StatusOK))

	server := httptest.NewServer(r)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL + "/drafts/content/%s/annotations")
	require.NoError(t, err)

	actual, err := client.SaveAnnotations(testCtx, testUuid, testAnnotations)
	assert.NoError(t, err)

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

	r := vestigo.NewRouter()
	r.Put("/drafts/content/:uuid/annotations", mockSaveAnnotations(t, testTid, testUuid, http.StatusCreated))

	server := httptest.NewServer(r)
	defer server.Close()

	client, err := NewAnnotationsClient(server.URL + "/drafts/content/%s/annotations")
	require.NoError(t, err)

	actual, err := client.SaveAnnotations(testCtx, testUuid, testAnnotations)
	assert.NoError(t, err)

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
	r.Put("/drafts/content/:uuid/annotations", mockSaveAnnotations(t, testTid, testUuid, http.StatusInternalServerError))

	server := httptest.NewServer(r)
	defer server.Close()

	annotationsUrl := server.URL + "/drafts/content/%s/annotations"
	client, err := NewAnnotationsClient(annotationsUrl)
	require.NoError(t, err)

	_, err = client.SaveAnnotations(testCtx, testUuid, testAnnotations)
	assert.EqualError(t, err, fmt.Sprintf("Write to %s returned a 500 status code", fmt.Sprintf(annotationsUrl, testUuid)))
}
