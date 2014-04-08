package ghttp_test

import (
	"bytes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"
	"io/ioutil"
	"net/http"
)

var _ = Describe("TestServer", func() {
	var (
		resp *http.Response
		err  error
		s    *Server
	)

	interceptFailures := func(f func()) []string {
		failures := []string{}
		RegisterFailHandler(func(message string, callerSkip ...int) {
			failures = append(failures, message)
		})
		f()
		RegisterFailHandler(Fail)
		return failures
	}

	BeforeEach(func() {
		s = NewServer()
	})

	AfterEach(func() {
		s.Close()
	})

	Describe("allowing unhandled requests", func() {
		Context("when true", func() {
			BeforeEach(func() {
				s.AllowUnhandledRequests = true
				s.UnhandledRequestStatusCode = http.StatusForbidden
				resp, err = http.Get(s.URL() + "/foo")
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("should allow unhandled requests and respond with the passed in status code", func() {
				Ω(err).ShouldNot(HaveOccurred())
				Ω(resp.StatusCode).Should(Equal(http.StatusForbidden))

				data, err := ioutil.ReadAll(resp.Body)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(data).Should(BeEmpty())
			})

			It("should record the requests", func() {
				Ω(s.ReceivedRequests()).Should(HaveLen(1))
				Ω(s.ReceivedRequests()[0].URL.Path).Should(Equal("/foo"))
			})
		})

		Context("when false", func() {
			It("should fail when attempting a request", func() {
				failures := interceptFailures(func() {
					http.Get(s.URL() + "/foo")
				})

				Ω(failures[0]).Should(ContainSubstring("Received Unhandled Request"))
			})
		})
	})

	Describe("Managing Handlers", func() {
		var called []string
		BeforeEach(func() {
			called = []string{}
			s.AppendHandlers(func(w http.ResponseWriter, req *http.Request) {
				called = append(called, "A")
			}, func(w http.ResponseWriter, req *http.Request) {
				called = append(called, "B")
			})
		})

		It("should call the appended handlers, in order, as requests come in", func() {
			http.Get(s.URL() + "/foo")
			Ω(called).Should(Equal([]string{"A"}))

			http.Get(s.URL() + "/foo")
			Ω(called).Should(Equal([]string{"A", "B"}))

			failures := interceptFailures(func() {
				http.Get(s.URL() + "/foo")
			})

			Ω(failures[0]).Should(ContainSubstring("Received Unhandled Request"))
		})

		Describe("Overwriting an existing handler", func() {
			BeforeEach(func() {
				s.SetHandler(0, func(w http.ResponseWriter, req *http.Request) {
					called = append(called, "C")
				})
			})

			It("should override the specified handler", func() {
				http.Get(s.URL() + "/foo")
				http.Get(s.URL() + "/foo")
				Ω(called).Should(Equal([]string{"C", "B"}))
			})
		})

		Describe("Getting an existing handler", func() {
			It("should return the handler func", func() {
				s.GetHandler(1)(nil, nil)
				Ω(called).Should(Equal([]string{"B"}))
			})
		})

		Describe("Wrapping an existing handler", func() {
			BeforeEach(func() {
				s.WrapHandler(0, func(w http.ResponseWriter, req *http.Request) {
					called = append(called, "C")
				})
			})

			It("should wrap the existing handler in a new handler", func() {
				http.Get(s.URL() + "/foo")
				http.Get(s.URL() + "/foo")
				Ω(called).Should(Equal([]string{"A", "C", "B"}))
			})
		})
	})

	Describe("Request Handlers", func() {
		Describe("VerifyRequest", func() {
			BeforeEach(func() {
				s.AppendHandlers(VerifyRequest("GET", "/foo"))
			})

			It("should verify the method, path", func() {
				resp, err = http.Get(s.URL() + "/foo?baz=bar")
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("should verify the method, path", func() {
				failures := interceptFailures(func() {
					http.Get(s.URL() + "/foo2")
				})
				Ω(failures).Should(HaveLen(1))
			})

			It("should verify the method, path", func() {
				failures := interceptFailures(func() {
					http.Post(s.URL()+"/foo", "application/json", nil)
				})
				Ω(failures).Should(HaveLen(1))
			})

			It("should also be possible to verify the rawQuery", func() {
				s.SetHandler(0, VerifyRequest("GET", "/foo", "baz=bar"))
				resp, err = http.Get(s.URL() + "/foo?baz=bar")
				Ω(err).ShouldNot(HaveOccurred())
			})
		})

		Describe("VerifyContentType", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyRequest("GET", "/foo"),
					VerifyContentType("application/octet-stream"),
				))
			})

			It("should verify the content type", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Ω(err).ShouldNot(HaveOccurred())
				req.Header.Set("Content-Type", "application/octet-stream")

				resp, err = http.DefaultClient.Do(req)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("should verify the content type", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Ω(err).ShouldNot(HaveOccurred())
				req.Header.Set("Content-Type", "application/json")

				failures := interceptFailures(func() {
					http.DefaultClient.Do(req)
				})
				Ω(failures).Should(HaveLen(1))
			})
		})

		Describe("Verify BasicAuth", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyRequest("GET", "/foo"),
					VerifyBasicAuth("bob", "password"),
				))
			})

			It("should verify basic auth", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Ω(err).ShouldNot(HaveOccurred())
				req.SetBasicAuth("bob", "password")

				resp, err = http.DefaultClient.Do(req)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("should verify basic auth", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Ω(err).ShouldNot(HaveOccurred())
				req.SetBasicAuth("bob", "bassword")

				failures := interceptFailures(func() {
					http.DefaultClient.Do(req)
				})
				Ω(failures).Should(HaveLen(1))
			})

		})

		Describe("VerifyHeader", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyRequest("GET", "/foo"),
					VerifyHeader(http.Header{
						"accept":        []string{"jpeg", "png"},
						"cache-control": []string{"omicron"},
						"Return-Path":   []string{"hobbiton"},
					}),
				))
			})

			It("should verify the headers", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Ω(err).ShouldNot(HaveOccurred())
				req.Header.Add("Accept", "jpeg")
				req.Header.Add("Accept", "png")
				req.Header.Add("Cache-Control", "omicron")
				req.Header.Add("return-path", "hobbiton")

				resp, err = http.DefaultClient.Do(req)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("should verify the headers", func() {
				req, err := http.NewRequest("GET", s.URL()+"/foo", nil)
				Ω(err).ShouldNot(HaveOccurred())
				req.Header.Add("Schmaccept", "jpeg")
				req.Header.Add("Schmaccept", "png")
				req.Header.Add("Cache-Control", "omicron")
				req.Header.Add("return-path", "hobbiton")

				failures := interceptFailures(func() {
					http.DefaultClient.Do(req)
				})
				Ω(failures).Should(HaveLen(1))
			})
		})

		Describe("VerifyJSON", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyRequest("POST", "/foo"),
					VerifyJSON(`{"a":3, "b":2}`),
				))
			})

			It("should verify the json body and the content type", func() {
				resp, err = http.Post(s.URL()+"/foo", "application/json", bytes.NewReader([]byte(`{"b":2, "a":3}`)))
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("should verify the json body and the content type", func() {
				failures := interceptFailures(func() {
					http.Post(s.URL()+"/foo", "application/json", bytes.NewReader([]byte(`{"b":2, "a":4}`)))
				})
				Ω(failures).Should(HaveLen(1))
			})
		})

		Describe("RespondWith", func() {
			BeforeEach(func() {
				s.AppendHandlers(CombineHandlers(
					VerifyRequest("POST", "/foo"),
					RespondWith(http.StatusCreated, "sweet"),
				))
			})

			It("should return the response", func() {
				resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(resp.StatusCode).Should(Equal(http.StatusCreated))

				body, err := ioutil.ReadAll(resp.Body)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(body).Should(Equal([]byte("sweet")))
			})
		})

		Describe("RespondWithPtr", func() {
			var code int
			var body string
			BeforeEach(func() {
				code = http.StatusOK
				body = "sweet"

				s.AppendHandlers(CombineHandlers(
					VerifyRequest("POST", "/foo"),
					RespondWithPtr(&code, &body),
				))
			})

			It("should return the response", func() {
				code = http.StatusCreated
				body = "tasty"
				resp, err = http.Post(s.URL()+"/foo", "application/json", nil)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(resp.StatusCode).Should(Equal(http.StatusCreated))

				body, err := ioutil.ReadAll(resp.Body)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(body).Should(Equal([]byte("tasty")))
			})
		})
	})
})
