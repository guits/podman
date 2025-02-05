package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run with --cgroup-parent", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootlessCgroupsV1("cgroup parent is not supported in cgroups v1")
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	Specify("valid --cgroup-parent using cgroupfs", func() {
		if !Containerized() {
			Skip("Must be containerized to run this test.")
		}
		cgroup := "/zzz"
		run := podmanTest.Podman([]string{"run", "--cgroupns=host", "--cgroup-parent", cgroup, fedoraMinimal, "cat", "/proc/self/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		ok, _ := run.GrepString(cgroup)
		Expect(ok).To(BeTrue())
	})

	Specify("no --cgroup-parent", func() {
		SkipIfRootless("FIXME This seems to be broken in rootless mode")
		cgroup := "/libpod_parent"
		if !Containerized() && podmanTest.CgroupManager != "cgroupfs" {
			cgroup = "/machine.slice"
		}
		run := podmanTest.Podman([]string{"run", "--cgroupns=host", fedoraMinimal, "cat", "/proc/self/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		ok, _ := run.GrepString(cgroup)
		Expect(ok).To(BeTrue())
	})

	Specify("always honor --cgroup-parent", func() {
		SkipIfCgroupV1("test not supported in cgroups v1")
		if Containerized() || podmanTest.CgroupManager == "cgroupfs" {
			Skip("Requires Systemd cgroup manager support")
		}
		if IsRemote() {
			Skip("Not supported for remote")
		}

		run := podmanTest.Podman([]string{"run", "-d", "--cgroupns=host", fedoraMinimal, "sleep", "100"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		cid := run.OutputToString()

		exec := podmanTest.Podman([]string{"exec", cid, "cat", "/proc/self/cgroup"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))

		containerCgroup := strings.TrimRight(strings.Replace(exec.OutputToString(), "0::", "", -1), "\n")

		content, err := ioutil.ReadFile(filepath.Join("/sys/fs/cgroup", containerCgroup, "cgroup.procs"))
		Expect(err).To(BeNil())

		// Move the container process to a sub cgroup
		subCgroupPath := filepath.Join(filepath.Join("/sys/fs/cgroup", containerCgroup, "old-container"))

		err = os.MkdirAll(subCgroupPath, 0755)
		Expect(err).To(BeNil())

		err = ioutil.WriteFile(filepath.Join(subCgroupPath, "cgroup.procs"), content, 0644)
		Expect(err).To(BeNil())

		cgroup := filepath.Dir(containerCgroup)

		run = podmanTest.Podman([]string{"--cgroup-manager=cgroupfs", "run", "-d", fmt.Sprintf("--cgroup-parent=%s", cgroup), fedoraMinimal, "sleep", "100"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))

		exec = podmanTest.Podman([]string{"exec", cid, "cat", "/proc/self/cgroup"})
		exec.WaitWithDefaultTimeout()
		Expect(exec).Should(Exit(0))
		cgroupEffective := filepath.Dir(strings.TrimRight(strings.Replace(exec.OutputToString(), "0::", "", -1), "\n"))

		Expect(cgroupEffective).To(Equal(cgroup))
	})

	Specify("valid --cgroup-parent using slice", func() {
		if Containerized() || podmanTest.CgroupManager == "cgroupfs" {
			Skip("Requires Systemd cgroup manager support")
		}
		cgroup := "aaaa.slice"
		run := podmanTest.Podman([]string{"run", "--cgroupns=host", "--cgroup-parent", cgroup, fedoraMinimal, "cat", "/proc/1/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))
		ok, _ := run.GrepString(cgroup)
		Expect(ok).To(BeTrue())
	})
})
