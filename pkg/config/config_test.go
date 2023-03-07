package config

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ty "github.com/spidernet-io/plugins/pkg/types"
	"k8s.io/utils/pointer"
)

var _ = Describe("config", func() {
	Context("Test ValidateRoutes", func() {
		It("ignore leading or trailing spaces", func() {
			subnet := []string{" 1.1.1.0/24", " 2.2.2.0/24 "}
			err := validateRoutes(subnet)
			Expect(err).NotTo(HaveOccurred())
		})

		It("invalid cidr return err", func() {
			overlaySubnet := []string{"abcd"}
			err := validateRoutes(overlaySubnet)
			Expect(err).To(HaveOccurred())
		})

		It("correct cidr config", func() {
			overlaySubnet := []string{"10.69.0.0/12", "fd00:10:244::/64"}
			err := validateRoutes(overlaySubnet)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Test ValidateRPFilterConfig", func() {
		It("no rp_filter config", func() {
			var config *ty.RPFilter
			validateRPFilterConfig(config)
			Expect(config).To(BeNil())
		})

		It("enable rp_filter but no value given, we give default value to it", func() {
			var config = &ty.RPFilter{
				Enable: pointer.Bool(true),
			}
			var want = &ty.RPFilter{
				Enable: pointer.Bool(true),
				Value:  0,
			}
			validateRPFilterConfig(config)
			Expect(config).To(Equal(want))
		})

		It("give value but disable rp_filter", func() {
			var config = &ty.RPFilter{
				Enable: nil,
				Value:  2,
			}
			var want = &ty.RPFilter{
				Enable: pointer.Bool(true),
				Value:  0,
			}
			validateRPFilterConfig(config)
			Expect(config).To(Equal(want))
		})

		It("value must be 0/1/2, if not we set it to 0", func() {
			var config = &ty.RPFilter{
				Enable: pointer.Bool(true),
				Value:  10,
			}
			var want = &ty.RPFilter{
				Enable: pointer.Bool(true),
				Value:  0,
			}
			validateRPFilterConfig(config)
			Expect(config).To(Equal(want))
		})

		It("correct rp_filter config", func() {
			var config = &ty.RPFilter{
				Enable: pointer.Bool(true),
				Value:  1,
			}
			var want = &ty.RPFilter{
				Enable: pointer.Bool(true),
				Value:  1,
			}
			validateRPFilterConfig(config)
			Expect(config).To(Equal(want))
		})
	})

	Context("Test validateHwPrefix", func() {
		It("mac_options is empty", func() {
			err := validateHwPrefix("")
			Expect(err).To(BeNil())
		})
		It("prefix is invalid return err", func() {
			err := validateHwPrefix("wrong mac")
			Expect(err.Error()).To(Equal("mac_prefix format should be match regex: [a-fA-F0-9]{2}[:][a-fA-F0-9]{2}, like '0a:1b'"))
		})
		It("enable and prefix is valid", func() {
			err := validateHwPrefix("0a:1b")
			Expect(err).To(BeNil())
		})
	})
})
