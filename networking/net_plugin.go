// Copyright 2015 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package networking

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	cnitypes "github.com/appc/cni/pkg/types"
	"github.com/hashicorp/errwrap"

	"github.com/coreos/rkt/common"
)

// TODO(eyakubovich): make this configurable in rkt.conf
const UserNetPluginsPath = "/usr/lib/rkt/plugins/net"
const BuiltinNetPluginsPath = "usr/lib/rkt/plugins/net"

func pluginErr(err error, output []byte) error {
	if _, ok := err.(*exec.ExitError); ok {
		emsg := cnitypes.Error{}
		if perr := json.Unmarshal(output, &emsg); perr != nil {
			return errwrap.Wrap(fmt.Errorf("netplugin failed but error parsing its diagnostic message %q", string(output)), perr)
		}
		details := ""
		if emsg.Details != "" {
			details = fmt.Sprintf("; %v", emsg.Details)
		}
		return fmt.Errorf("%v%v", emsg.Msg, details)
	}

	return err
}

func (e *podEnv) netPluginAdd(n *activeNet, netns string) (ip, hostIP net.IP, err error) {
	output, err := e.execNetPlugin("ADD", n, netns)
	if err != nil {
		return nil, nil, pluginErr(err, output)
	}

	pr := cnitypes.Result{}
	if err = json.Unmarshal(output, &pr); err != nil {
		return nil, nil, errwrap.Wrap(fmt.Errorf("error parsing %q result", n.conf.Name), err)
	}

	if pr.IP4 == nil {
		return nil, nil, fmt.Errorf("net-plugin returned no IPv4 configuration")
	}

	return pr.IP4.IP.IP, pr.IP4.Gateway, nil
}

func (e *podEnv) netPluginDel(n *activeNet, netns string) error {
	output, err := e.execNetPlugin("DEL", n, netns)
	if err != nil {
		return pluginErr(err, output)
	}
	return nil
}

func (e *podEnv) pluginPaths() []string {
	// try 3rd-party path first
	return []string{
		filepath.Join(e.localConfig, UserNetPathSuffix),
		UserNetPluginsPath,
		filepath.Join(common.Stage1RootfsPath(e.podRoot), BuiltinNetPluginsPath),
	}
}

func (e *podEnv) findNetPlugin(plugin string) string {
	for _, p := range e.pluginPaths() {
		fullname := filepath.Join(p, plugin)
		if fi, err := os.Stat(fullname); err == nil && fi.Mode().IsRegular() {
			return fullname
		}
	}

	return ""
}

func envVars(vars [][2]string) []string {
	env := os.Environ()

	for _, kv := range vars {
		env = append(env, strings.Join(kv[:], "="))
	}

	return env
}

func (e *podEnv) execNetPlugin(cmd string, n *activeNet, netns string) ([]byte, error) {
	pluginPath := e.findNetPlugin(n.conf.Type)
	if pluginPath == "" {
		return nil, fmt.Errorf("Could not find plugin %q", n.conf.Type)
	}

	vars := [][2]string{
		{"CNI_COMMAND", cmd},
		{"CNI_CONTAINERID", e.podID.String()},
		{"CNI_NETNS", netns},
		{"CNI_ARGS", n.runtime.Args},
		{"CNI_IFNAME", n.runtime.IfName},
		{"CNI_PATH", strings.Join(e.pluginPaths(), ":")},
	}

	stdin := bytes.NewBuffer(n.confBytes)
	stdout := &bytes.Buffer{}

	c := exec.Cmd{
		Path:   pluginPath,
		Args:   []string{pluginPath},
		Env:    envVars(vars),
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: os.Stderr,
	}

	err := c.Run()
	return stdout.Bytes(), err
}
