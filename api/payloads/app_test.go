package payloads_test

import (
	"code.cloudfoundry.org/korifi/api/payloads"
	"code.cloudfoundry.org/korifi/api/repositories"
	"code.cloudfoundry.org/korifi/tools"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
)

var _ = Describe("AppList", func() {
	Describe("Validation", func() {
		DescribeTable("valid query",
			func(query string, expectedAppList payloads.AppList) {
				actualAppList, decodeErr := decodeQuery[payloads.AppList](query)

				Expect(decodeErr).NotTo(HaveOccurred())
				Expect(*actualAppList).To(Equal(expectedAppList))
			},

			Entry("names", "names=name", payloads.AppList{Names: "name"}),
			Entry("guids", "guids=guid", payloads.AppList{GUIDs: "guid"}),
			Entry("space_guids", "space_guids=space_guid", payloads.AppList{SpaceGUIDs: "space_guid"}),
			Entry("order_by created_at", "order_by=created_at", payloads.AppList{OrderBy: "created_at"}),
			Entry("order_by -created_at", "order_by=-created_at", payloads.AppList{OrderBy: "-created_at"}),
			Entry("order_by updated_at", "order_by=updated_at", payloads.AppList{OrderBy: "updated_at"}),
			Entry("order_by -updated_at", "order_by=-updated_at", payloads.AppList{OrderBy: "-updated_at"}),
			Entry("order_by name", "order_by=name", payloads.AppList{OrderBy: "name"}),
			Entry("order_by -name", "order_by=-name", payloads.AppList{OrderBy: "-name"}),
			Entry("order_by state", "order_by=state", payloads.AppList{OrderBy: "state"}),
			Entry("order_by -state", "order_by=-state", payloads.AppList{OrderBy: "-state"}),
			Entry("label_selector=foo", "label_selector=foo", payloads.AppList{LabelSelector: "foo"}),
			Entry("page=3", "page=3", payloads.AppList{Pagination: payloads.Pagination{Page: "3"}}),
		)

		DescribeTable("invalid query",
			func(query string, expectedErrMsg string) {
				_, decodeErr := decodeQuery[payloads.AppList](query)
				Expect(decodeErr).To(MatchError(ContainSubstring(expectedErrMsg)))
			},
			Entry("invalid order_by", "order_by=foo", "value must be one of"),
			Entry("per_page is not a number", "per_page=foo", "value must be an integer"),
		)
	})

	Describe("ToMessage", func() {
		var (
			appList payloads.AppList
			message repositories.ListAppsMessage
		)

		BeforeEach(func() {
			appList = payloads.AppList{
				Names:         "n1,n2",
				GUIDs:         "g1,g2",
				SpaceGUIDs:    "s1,s2",
				OrderBy:       "created_at",
				LabelSelector: "foo=bar",
				Pagination: payloads.Pagination{
					PerPage: "20",
					Page:    "1",
				},
			}
		})

		JustBeforeEach(func() {
			message = appList.ToMessage()
		})

		It("translates to repository message", func() {
			Expect(message).To(Equal(repositories.ListAppsMessage{
				Names:         []string{"n1", "n2"},
				Guids:         []string{"g1", "g2"},
				SpaceGUIDs:    []string{"s1", "s2"},
				OrderBy:       "created_at",
				LabelSelector: "foo=bar",
				Pagination: repositories.Pagination{
					Page:    1,
					PerPage: 20,
				},
			}))
		})
	})
})

var _ = Describe("App payload validation", func() {
	var validatorErr error

	Describe("AppCreate", func() {
		Describe("Decoding", func() {
			var (
				payload        payloads.AppCreate
				decodedPayload *payloads.AppCreate
			)

			BeforeEach(func() {
				payload = payloads.AppCreate{
					Name: "my-app",
					Relationships: &payloads.AppRelationships{
						Space: &payloads.Relationship{
							Data: &payloads.RelationshipData{
								GUID: "app-guid",
							},
						},
					},
				}

				decodedPayload = new(payloads.AppCreate)
			})

			JustBeforeEach(func() {
				validatorErr = validator.DecodeAndValidateJSONPayload(createJSONRequest(payload), decodedPayload)
			})

			It("succeeds", func() {
				Expect(validatorErr).NotTo(HaveOccurred())
				Expect(decodedPayload).To(gstruct.PointTo(Equal(payload)))
			})

			When("name is not set", func() {
				BeforeEach(func() {
					payload.Name = ""
				})

				It("returns an error", func() {
					expectUnprocessableEntityError(validatorErr, "name cannot be blank")
				})
			})

			When("name is invalid", func() {
				BeforeEach(func() {
					payload.Name = "!@#"
				})

				It("returns an error", func() {
					expectUnprocessableEntityError(validatorErr, "name must consist only of letters, numbers, underscores and dashes")
				})
			})

			When("lifecycle is invalid", func() {
				BeforeEach(func() {
					payload.Lifecycle = &payloads.Lifecycle{}
				})

				It("returns an unprocessable entity error", func() {
					expectUnprocessableEntityError(validatorErr, "lifecycle.type cannot be blank")
				})
			})

			When("relationships are not set", func() {
				BeforeEach(func() {
					payload.Relationships = nil
				})

				It("returns an unprocessable entity error", func() {
					expectUnprocessableEntityError(validatorErr, "relationships is required")
				})
			})

			When("relationships space is not set", func() {
				BeforeEach(func() {
					payload.Relationships.Space = nil
				})

				It("returns an unprocessable entity error", func() {
					expectUnprocessableEntityError(validatorErr, "relationships.space is required")
				})
			})

			When("metadata is invalid", func() {
				BeforeEach(func() {
					payload.Metadata = payloads.Metadata{
						Labels: map[string]string{
							"foo.cloudfoundry.org/bar": "jim",
						},
					}
				})

				It("returns an appropriate error", func() {
					expectUnprocessableEntityError(validatorErr, "label/annotation key cannot use the cloudfoundry.org domain")
				})
			})
		})

		Describe("ToAppCreateMessage", func() {
			var (
				payload     payloads.AppCreate
				repoMessage repositories.CreateAppMessage
			)

			BeforeEach(func() {
				payload = payloads.AppCreate{
					Name: "app-name",
					EnvironmentVariables: map[string]string{
						"foo": "bar",
					},
					Relationships: &payloads.AppRelationships{
						Space: &payloads.Relationship{
							Data: &payloads.RelationshipData{
								GUID: "space-guid",
							},
						},
					},
					Metadata: payloads.Metadata{
						Labels: map[string]string{
							"l1": "v1",
						},
					},
				}
			})

			JustBeforeEach(func() {
				repoMessage = payload.ToAppCreateMessage()
			})

			It("creates an app create message with default lifecycle", func() {
				Expect(repoMessage).To(Equal(repositories.CreateAppMessage{
					Name:      "app-name",
					SpaceGUID: "space-guid",
					State:     repositories.StoppedState,
					Lifecycle: repositories.Lifecycle{
						Type: "buildpack",
						Data: repositories.LifecycleData{
							Stack: "cflinuxfs3",
						},
					},
					EnvironmentVariables: map[string]string{
						"foo": "bar",
					},
					Metadata: repositories.Metadata{
						Labels: map[string]string{
							"l1": "v1",
						},
					},
				}))
			})

			When("the lifecycle is buildpack", func() {
				BeforeEach(func() {
					payload.Lifecycle = &payloads.Lifecycle{
						Type: "buildpack",
						Data: &payloads.LifecycleData{
							Buildpacks: []string{"my-bp"},
							Stack:      "my-stack",
						},
					}
				})

				It("sets the lifecycle to the repo message", func() {
					Expect(repoMessage.Lifecycle).To(Equal(repositories.Lifecycle{
						Type: "buildpack",
						Data: repositories.LifecycleData{
							Buildpacks: []string{"my-bp"},
							Stack:      "my-stack",
						},
					}))
				})
			})

			When("the lifecycle is docker", func() {
				BeforeEach(func() {
					payload.Lifecycle = &payloads.Lifecycle{
						Type: "docker",
						Data: &payloads.LifecycleData{},
					}
				})

				It("sets the lifecycle to the repo message", func() {
					Expect(repoMessage.Lifecycle).To(Equal(repositories.Lifecycle{
						Type: "docker",
						Data: repositories.LifecycleData{},
					}))
				})
			})
		})
	})

	Describe("AppPatch", func() {
		var (
			payload        payloads.AppPatch
			decodedPayload *payloads.AppPatch
		)

		BeforeEach(func() {
			payload = payloads.AppPatch{
				Name: "bob",
				Lifecycle: &payloads.LifecyclePatch{
					Type: "buildpack",
					Data: &payloads.LifecycleDataPatch{
						Buildpacks: &[]string{"buildpack"},
						Stack:      "mystack",
					},
				},
				Metadata: payloads.MetadataPatch{
					Labels: map[string]*string{
						"foo": tools.PtrTo("bar"),
					},
					Annotations: map[string]*string{
						"example.org/jim": tools.PtrTo("hello"),
					},
				},
			}

			decodedPayload = new(payloads.AppPatch)
		})

		Describe("validation", func() {
			JustBeforeEach(func() {
				validatorErr = validator.DecodeAndValidateJSONPayload(createJSONRequest(payload), decodedPayload)
			})

			It("succeeds", func() {
				Expect(validatorErr).NotTo(HaveOccurred())
				Expect(decodedPayload).To(gstruct.PointTo(Equal(payload)))
			})

			When("metadata is invalid", func() {
				BeforeEach(func() {
					payload.Metadata = payloads.MetadataPatch{
						Labels: map[string]*string{
							"foo.cloudfoundry.org/bar": tools.PtrTo("jim"),
						},
					}
				})

				It("returns an appropriate error", func() {
					expectUnprocessableEntityError(validatorErr, "label/annotation key cannot use the cloudfoundry.org domain")
				})
			})

			When("name is invalid", func() {
				BeforeEach(func() {
					payload.Name = "!@#"
				})

				It("returns an error", func() {
					expectUnprocessableEntityError(validatorErr, "name must consist only of letters, numbers, underscores and dashes")
				})
			})

			When("lifecycle data is not set", func() {
				BeforeEach(func() {
					payload.Lifecycle.Data = nil
				})

				It("returns an error", func() {
					expectUnprocessableEntityError(validatorErr, "lifecycle.data is required")
				})
			})
		})

		Describe("To Message", func() {
			var msg repositories.PatchAppMessage

			JustBeforeEach(func() {
				msg = payload.ToMessage("app-guid", "space-guid")
			})

			It("creates the right message", func() {
				Expect(msg).To(Equal(repositories.PatchAppMessage{
					Name:      "bob",
					AppGUID:   "app-guid",
					SpaceGUID: "space-guid",
					Lifecycle: &repositories.LifecyclePatch{
						Type: tools.PtrTo("buildpack"),
						Data: &repositories.LifecycleDataPatch{
							Buildpacks: &[]string{"buildpack"},
							Stack:      "mystack",
						},
					},
					EnvironmentVariables: nil,
					MetadataPatch: repositories.MetadataPatch{
						Annotations: map[string]*string{"example.org/jim": tools.PtrTo("hello")},
						Labels:      map[string]*string{"foo": tools.PtrTo("bar")},
					},
				}))
			})

			When("lifecycle is not set", func() {
				BeforeEach(func() {
					payload.Lifecycle = nil
				})

				It("has lifecycle as nil", func() {
					Expect(msg.Lifecycle).To(BeNil())
				})
			})

			When("lifecycle.data is not set", func() {
				BeforeEach(func() {
					payload.Lifecycle.Data = nil
				})

				It("has lifecycle.data as nil", func() {
					Expect(msg.Lifecycle.Data).To(BeNil())
				})
			})

			When("lifecycle.data is empty", func() {
				BeforeEach(func() {
					payload.Lifecycle.Data = &payloads.LifecycleDataPatch{}
				})

				It("has empty lifecycle.data", func() {
					Expect(*msg.Lifecycle.Data).To(BeZero())
				})
			})

			When("only buildpacks are set", func() {
				BeforeEach(func() {
					payload.Lifecycle.Data = &payloads.LifecycleDataPatch{
						Buildpacks: &[]string{"mystack"},
					}
				})

				It("has stack empty", func() {
					Expect(msg.Lifecycle.Data.Stack).To(BeEmpty())
				})
			})

			When("only stack is set", func() {
				BeforeEach(func() {
					payload.Lifecycle.Data = &payloads.LifecycleDataPatch{
						Stack: "mystack",
					}
				})

				It("has buildpacks nil", func() {
					Expect(msg.Lifecycle.Data.Buildpacks).To(BeNil())
				})
			})
		})
	})

	Describe("AppSetCurrentDroplet", func() {
		var (
			payload        payloads.AppSetCurrentDroplet
			decodedPayload *payloads.AppSetCurrentDroplet
		)

		BeforeEach(func() {
			payload = payloads.AppSetCurrentDroplet{
				Relationship: payloads.Relationship{
					Data: &payloads.RelationshipData{
						GUID: "the-guid",
					},
				},
			}

			decodedPayload = new(payloads.AppSetCurrentDroplet)
		})

		JustBeforeEach(func() {
			validatorErr = validator.DecodeAndValidateJSONPayload(createJSONRequest(payload), decodedPayload)
		})

		It("succeeds", func() {
			Expect(validatorErr).NotTo(HaveOccurred())
			Expect(decodedPayload).To(gstruct.PointTo(Equal(payload)))
		})

		When("relationship is invalid", func() {
			BeforeEach(func() {
				payload.Relationship = payloads.Relationship{}
			})

			It("returns an appropriate error", func() {
				expectUnprocessableEntityError(validatorErr, "data is required")
			})
		})
	})

	Describe("AppPatchEnvVars", func() {
		var (
			payload        payloads.AppPatchEnvVars
			decodedPayload *payloads.AppPatchEnvVars
		)

		BeforeEach(func() {
			payload = payloads.AppPatchEnvVars{
				Var: map[string]interface{}{
					"foo": "bar",
				},
			}

			decodedPayload = new(payloads.AppPatchEnvVars)
		})

		JustBeforeEach(func() {
			validatorErr = validator.DecodeAndValidateJSONPayload(createJSONRequest(payload), decodedPayload)
		})

		It("succeeds", func() {
			Expect(validatorErr).NotTo(HaveOccurred())
			Expect(decodedPayload).To(gstruct.PointTo(Equal(payload)))
		})

		When("it contains a 'PORT' key", func() {
			BeforeEach(func() {
				payload.Var["PORT"] = "2222"
			})

			It("returns an appropriate error", func() {
				expectUnprocessableEntityError(validatorErr, "value PORT is not allowed")
			})
		})

		When("it contains a key with prefix 'VCAP_'", func() {
			BeforeEach(func() {
				payload.Var["VCAP_foo"] = "bar"
			})

			It("returns an appropriate error", func() {
				expectUnprocessableEntityError(validatorErr, "prefix VCAP_ is not allowed")
			})
		})

		When("it contains a key with prefix 'VMC_'", func() {
			BeforeEach(func() {
				payload.Var["VMC_foo"] = "bar"
			})

			It("returns an appropriate error", func() {
				expectUnprocessableEntityError(validatorErr, "prefix VMC_ is not allowed")
			})
		})
	})
})

var _ = Describe("AppRoutesList", func() {
	Describe("Validation", func() {
		DescribeTable("valid query",
			func(query string, expectedAppRoutesList payloads.AppRoutesList) {
				actualAppRoutesList, decodeErr := decodeQuery[payloads.AppRoutesList](query)

				Expect(decodeErr).NotTo(HaveOccurred())
				Expect(*actualAppRoutesList).To(Equal(expectedAppRoutesList))
			},

			Entry("order_by created_at", "order_by=created_at", payloads.AppRoutesList{OrderBy: "created_at"}),
			Entry("order_by -created_at", "order_by=-created_at", payloads.AppRoutesList{OrderBy: "-created_at"}),
			Entry("order_by updated_at", "order_by=updated_at", payloads.AppRoutesList{OrderBy: "updated_at"}),
			Entry("order_by -updated_at", "order_by=-updated_at", payloads.AppRoutesList{OrderBy: "-updated_at"}),
			Entry("page=3", "page=3", payloads.AppRoutesList{Pagination: payloads.Pagination{Page: "3"}}),
		)

		DescribeTable("invalid query",
			func(query string, expectedErrMsg string) {
				_, decodeErr := decodeQuery[payloads.AppRoutesList](query)
				Expect(decodeErr).To(MatchError(ContainSubstring(expectedErrMsg)))
			},
			Entry("invalid order_by", "order_by=foo", "value must be one of"),
			Entry("per_page is not a number", "per_page=foo", "value must be an integer"),
		)
	})

	Describe("ToMessage", func() {
		var (
			appList payloads.AppRoutesList
			message repositories.ListRoutesMessage
		)

		BeforeEach(func() {
			appList = payloads.AppRoutesList{
				OrderBy: "created_at",
				Pagination: payloads.Pagination{
					PerPage: "20",
					Page:    "1",
				},
			}
		})

		JustBeforeEach(func() {
			message = appList.ToMessage("app-guid")
		})

		It("translates to repository message", func() {
			Expect(message).To(Equal(repositories.ListRoutesMessage{
				AppGUIDs: []string{"app-guid"},
				OrderBy:  "created_at",
				Pagination: repositories.Pagination{
					Page:    1,
					PerPage: 20,
				},
			}))
		})
	})
})

var _ = Describe("AppDropletList", func() {
	DescribeTable("valid query",
		func(query string, expectedDropletList payloads.AppDropletsList) {
			actualDropletList, decodeErr := decodeQuery[payloads.AppDropletsList](query)

			Expect(decodeErr).NotTo(HaveOccurred())
			Expect(*actualDropletList).To(Equal(expectedDropletList))
		},
		Entry("guids", "guids=guid", payloads.AppDropletsList{GUIDs: "guid"}),
		Entry("order_by created_at", "order_by=created_at", payloads.AppDropletsList{OrderBy: "created_at"}),
		Entry("order_by -created_at", "order_by=-created_at", payloads.AppDropletsList{OrderBy: "-created_at"}),
		Entry("order_by updated_at", "order_by=updated_at", payloads.AppDropletsList{OrderBy: "updated_at"}),
		Entry("order_by -updated_at", "order_by=-updated_at", payloads.AppDropletsList{OrderBy: "-updated_at"}),
		Entry("pagination", "page=3", payloads.AppDropletsList{Pagination: payloads.Pagination{Page: "3"}}),
	)

	DescribeTable("invalid query",
		func(query string, expectedErrMsg string) {
			_, decodeErr := decodeQuery[payloads.AppDropletsList](query)
			Expect(decodeErr).To(MatchError(ContainSubstring(expectedErrMsg)))
		},
		Entry("invalid order_by", "order_by=foo", "value must be one of"),
		Entry("invalid parameter", "foo=bar", "unsupported query parameter: foo"),
		Entry("invalid pagination", "per_page=foo", "value must be an integer"),
	)

	Describe("ToMessage", func() {
		It("translates to repo message", func() {
			dropletList := payloads.AppDropletsList{
				GUIDs:   "g1,g2",
				OrderBy: "created_at",
				Pagination: payloads.Pagination{
					PerPage: "3",
					Page:    "2",
				},
			}
			Expect(dropletList.ToMessage("ag1")).To(Equal(repositories.ListDropletsMessage{
				GUIDs:    []string{"g1", "g2"},
				AppGUIDs: []string{"ag1"},
				OrderBy:  "created_at",
				Pagination: repositories.Pagination{
					Page:    2,
					PerPage: 3,
				},
			}))
		})
	})
})

var _ = Describe("AppRoutesList", func() {
	Describe("Validation", func() {
		DescribeTable("valid query",
			func(query string, expectedAppRoutesList payloads.AppRoutesList) {
				actualAppRoutesList, decodeErr := decodeQuery[payloads.AppRoutesList](query)

				Expect(decodeErr).NotTo(HaveOccurred())
				Expect(*actualAppRoutesList).To(Equal(expectedAppRoutesList))
			},

			Entry("order_by created_at", "order_by=created_at", payloads.AppRoutesList{OrderBy: "created_at"}),
			Entry("order_by -created_at", "order_by=-created_at", payloads.AppRoutesList{OrderBy: "-created_at"}),
			Entry("order_by updated_at", "order_by=updated_at", payloads.AppRoutesList{OrderBy: "updated_at"}),
			Entry("order_by -updated_at", "order_by=-updated_at", payloads.AppRoutesList{OrderBy: "-updated_at"}),
			Entry("page=3", "page=3", payloads.AppRoutesList{Pagination: payloads.Pagination{Page: "3"}}),
		)

		DescribeTable("invalid query",
			func(query string, expectedErrMsg string) {
				_, decodeErr := decodeQuery[payloads.AppRoutesList](query)
				Expect(decodeErr).To(MatchError(ContainSubstring(expectedErrMsg)))
			},
			Entry("invalid order_by", "order_by=foo", "value must be one of"),
			Entry("per_page is not a number", "per_page=foo", "value must be an integer"),
		)
	})

	Describe("ToMessage", func() {
		var (
			appList payloads.AppRoutesList
			message repositories.ListRoutesMessage
		)

		BeforeEach(func() {
			appList = payloads.AppRoutesList{
				OrderBy: "created_at",
				Pagination: payloads.Pagination{
					PerPage: "20",
					Page:    "1",
				},
			}
		})

		JustBeforeEach(func() {
			message = appList.ToMessage("app-guid")
		})

		It("translates to repository message", func() {
			Expect(message).To(Equal(repositories.ListRoutesMessage{
				AppGUIDs: []string{"app-guid"},
				OrderBy:  "created_at",
				Pagination: repositories.Pagination{
					Page:    1,
					PerPage: 20,
				},
			}))
		})
	})
})

var _ = Describe("AppDropletList", func() {
	DescribeTable("valid query",
		func(query string, expectedDropletList payloads.AppDropletsList) {
			actualDropletList, decodeErr := decodeQuery[payloads.AppDropletsList](query)

			Expect(decodeErr).NotTo(HaveOccurred())
			Expect(*actualDropletList).To(Equal(expectedDropletList))
		},
		Entry("guids", "guids=guid", payloads.AppDropletsList{GUIDs: "guid"}),
		Entry("order_by created_at", "order_by=created_at", payloads.AppDropletsList{OrderBy: "created_at"}),
		Entry("order_by -created_at", "order_by=-created_at", payloads.AppDropletsList{OrderBy: "-created_at"}),
		Entry("order_by updated_at", "order_by=updated_at", payloads.AppDropletsList{OrderBy: "updated_at"}),
		Entry("order_by -updated_at", "order_by=-updated_at", payloads.AppDropletsList{OrderBy: "-updated_at"}),
		Entry("pagination", "page=3", payloads.AppDropletsList{Pagination: payloads.Pagination{Page: "3"}}),
	)

	DescribeTable("invalid query",
		func(query string, expectedErrMsg string) {
			_, decodeErr := decodeQuery[payloads.AppDropletsList](query)
			Expect(decodeErr).To(MatchError(ContainSubstring(expectedErrMsg)))
		},
		Entry("invalid order_by", "order_by=foo", "value must be one of"),
		Entry("invalid parameter", "foo=bar", "unsupported query parameter: foo"),
		Entry("invalid pagination", "per_page=foo", "value must be an integer"),
	)

	Describe("ToMessage", func() {
		It("translates to repo message", func() {
			dropletList := payloads.AppDropletsList{
				GUIDs:   "g1,g2",
				OrderBy: "created_at",
				Pagination: payloads.Pagination{
					PerPage: "3",
					Page:    "2",
				},
			}
			Expect(dropletList.ToMessage("ag1")).To(Equal(repositories.ListDropletsMessage{
				GUIDs:    []string{"g1", "g2"},
				AppGUIDs: []string{"ag1"},
				OrderBy:  "created_at",
				Pagination: repositories.Pagination{
					Page:    2,
					PerPage: 3,
				},
			}))
		})
	})
})

var _ = Describe("AppPackageList", func() {
	DescribeTable("valid query",
		func(query string, expectedDropletList payloads.AppPackagesList) {
			actualDropletList, decodeErr := decodeQuery[payloads.AppPackagesList](query)

			Expect(decodeErr).NotTo(HaveOccurred())
			Expect(*actualDropletList).To(Equal(expectedDropletList))
		},
		Entry("order_by created_at", "order_by=created_at", payloads.AppPackagesList{OrderBy: "created_at"}),
		Entry("order_by -created_at", "order_by=-created_at", payloads.AppPackagesList{OrderBy: "-created_at"}),
		Entry("order_by updated_at", "order_by=updated_at", payloads.AppPackagesList{OrderBy: "updated_at"}),
		Entry("order_by -updated_at", "order_by=-updated_at", payloads.AppPackagesList{OrderBy: "-updated_at"}),
		Entry("pagination", "page=3", payloads.AppPackagesList{Pagination: payloads.Pagination{Page: "3"}}),
	)

	DescribeTable("invalid query",
		func(query string, expectedErrMsg string) {
			_, decodeErr := decodeQuery[payloads.AppPackagesList](query)
			Expect(decodeErr).To(MatchError(ContainSubstring(expectedErrMsg)))
		},
		Entry("invalid order_by", "order_by=foo", "value must be one of"),
		Entry("invalid parameter", "foo=bar", "unsupported query parameter: foo"),
		Entry("invalid pagination", "per_page=foo", "value must be an integer"),
	)

	Describe("ToMessage", func() {
		It("translates to repo message", func() {
			dropletList := payloads.AppPackagesList{
				OrderBy: "created_at",
				Pagination: payloads.Pagination{
					PerPage: "3",
					Page:    "2",
				},
			}
			Expect(dropletList.ToMessage("ag1")).To(Equal(repositories.ListPackagesMessage{
				AppGUIDs: []string{"ag1"},
				OrderBy:  "created_at",
				Pagination: repositories.Pagination{
					Page:    2,
					PerPage: 3,
				},
			}))
		})
	})
})

var _ = Describe("AppProcessList", func() {
	DescribeTable("valid query",
		func(query string, expectedDropletList payloads.AppProcessList) {
			actualDropletList, decodeErr := decodeQuery[payloads.AppProcessList](query)

			Expect(decodeErr).NotTo(HaveOccurred())
			Expect(*actualDropletList).To(Equal(expectedDropletList))
		},
		Entry("order_by created_at", "order_by=created_at", payloads.AppProcessList{OrderBy: "created_at"}),
		Entry("order_by -created_at", "order_by=-created_at", payloads.AppProcessList{OrderBy: "-created_at"}),
		Entry("order_by updated_at", "order_by=updated_at", payloads.AppProcessList{OrderBy: "updated_at"}),
		Entry("order_by -updated_at", "order_by=-updated_at", payloads.AppProcessList{OrderBy: "-updated_at"}),
		Entry("pagination", "page=3", payloads.AppProcessList{Pagination: payloads.Pagination{Page: "3"}}),
	)

	DescribeTable("invalid query",
		func(query string, expectedErrMsg string) {
			_, decodeErr := decodeQuery[payloads.AppProcessList](query)
			Expect(decodeErr).To(MatchError(ContainSubstring(expectedErrMsg)))
		},
		Entry("invalid order_by", "order_by=foo", "value must be one of"),
		Entry("invalid parameter", "foo=bar", "unsupported query parameter: foo"),
		Entry("invalid pagination", "per_page=foo", "value must be an integer"),
	)

	Describe("ToMessage", func() {
		It("translates to repo message", func() {
			dropletList := payloads.AppProcessList{
				OrderBy: "created_at",
				Pagination: payloads.Pagination{
					PerPage: "3",
					Page:    "2",
				},
			}
			Expect(dropletList.ToMessage("ag1")).To(Equal(repositories.ListProcessesMessage{
				AppGUIDs: []string{"ag1"},
				OrderBy:  "created_at",
				Pagination: repositories.Pagination{
					Page:    2,
					PerPage: 3,
				},
			}))
		})
	})
})
