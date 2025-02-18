package server_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/cri-o/cri-o/internal/oci"
	"github.com/go-zoo/bone"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = t.Describe("Inspect", func() {
	var (
		recorder *httptest.ResponseRecorder
		mux      *bone.Mux
	)

	// Prepare the sut
	BeforeEach(func() {
		beforeEach()
		setupSUT()

		recorder = httptest.NewRecorder()
		mux = sut.GetInfoMux(false)
		Expect(mux).NotTo(BeNil())
		Expect(recorder).NotTo(BeNil())
	})
	AfterEach(afterEach)

	t.Describe("GetInfoMux", func() {
		It("should succeed with /info route", func() {
			// Given
			// When
			request, err := http.NewRequest("GET", "/info", http.NoBody)
			mux.ServeHTTP(recorder, request)

			// Then
			Expect(err).To(BeNil())
			Expect(request).NotTo(BeNil())
			Expect(recorder.Code).To(BeEquivalentTo(http.StatusOK))
		})

		It("should succeed with valid /containers route", func() {
			// Given
			Expect(sut.AddSandbox(testSandbox)).To(BeNil())
			testContainer.SetStateAndSpoofPid(&oci.ContainerState{})
			Expect(testSandbox.SetInfraContainer(testContainer)).To(BeNil())
			sut.AddContainer(testContainer)

			// When
			request, err := http.NewRequest("GET",
				"/containers/"+testContainer.ID(), http.NoBody)
			mux.ServeHTTP(recorder, request)

			// Then
			Expect(err).To(BeNil())
			Expect(request).NotTo(BeNil())
			Expect(recorder.Code).To(BeEquivalentTo(http.StatusOK))
		})

		It("should fail if sandbox not found on /containers route", func() {
			// Given
			Expect(sut.AddSandbox(testSandbox)).To(BeNil())
			testContainer.SetStateAndSpoofPid(&oci.ContainerState{})
			Expect(testSandbox.SetInfraContainer(testContainer)).To(BeNil())
			sut.AddContainer(testContainer)
			Expect(sut.RemoveSandbox(testSandbox.ID())).To(BeNil())

			// When
			request, err := http.NewRequest("GET",
				"/containers/"+testContainer.ID(), http.NoBody)
			mux.ServeHTTP(recorder, request)

			// Then
			Expect(err).To(BeNil())
			Expect(request).NotTo(BeNil())
			Expect(recorder.Code).To(BeEquivalentTo(http.StatusNotFound))
		})

		It("should fail if container state is nil on /containers route", func() {
			// Given
			Expect(sut.AddSandbox(testSandbox)).To(BeNil())
			Expect(testSandbox.SetInfraContainer(testContainer)).To(BeNil())
			testContainer.SetState(nil)
			sut.AddContainer(testContainer)

			// When
			request, err := http.NewRequest("GET",
				"/containers/"+testContainer.ID(), http.NoBody)
			mux.ServeHTTP(recorder, request)

			// Then
			Expect(err).To(BeNil())
			Expect(request).NotTo(BeNil())
			Expect(recorder.Code).
				To(BeEquivalentTo(http.StatusInternalServerError))
		})

		It("should fail with empty with /containers route", func() {
			// Given
			// When
			request, err := http.NewRequest("GET", "/containers", http.NoBody)
			mux.ServeHTTP(recorder, request)

			// Then
			Expect(err).To(BeNil())
			Expect(request).NotTo(BeNil())
			Expect(recorder.Code).To(BeEquivalentTo(http.StatusNotFound))
		})

		It("should fail with invalid container ID on /containers route", func() {
			// Given
			// When
			request, err := http.NewRequest("GET", "/containers/123", http.NoBody)
			mux.ServeHTTP(recorder, request)

			// Then
			Expect(err).To(BeNil())
			Expect(request).NotTo(BeNil())
			Expect(recorder.Code).To(BeEquivalentTo(http.StatusNotFound))
		})
	})
})
