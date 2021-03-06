package containers

import (
	"os"
	"reflect"
	"strconv"

	v1alpha1 "github.com/interconnectedcloud/qdrouterd-operator/pkg/apis/interconnectedcloud/v1alpha1"
	"github.com/interconnectedcloud/qdrouterd-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	log = logf.Log.WithName("containers")
)

func containerPortsForListeners(listeners []v1alpha1.Listener) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{}
	for _, listener := range listeners {
		ports = append(ports, corev1.ContainerPort{
			Name:          nameForListener(&listener),
			ContainerPort: listener.Port,
		})
	}
	return ports
}

func containerPortsForQdrouterd(m *v1alpha1.Qdrouterd) []corev1.ContainerPort {
	ports := containerPortsForListeners(m.Spec.Listeners)
	ports = append(ports, containerPortsForListeners(m.Spec.InterRouterListeners)...)
	return ports
}

func nameForListener(l *v1alpha1.Listener) string {
	if l.Name == "" {
		return "port-" + strconv.Itoa(int(l.Port))
	} else {
		return l.Name
	}
}

func containerEnvVarsForQdrouterd(m *v1alpha1.Qdrouterd) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}
	envVars = append(envVars, corev1.EnvVar{Name: "APPLICATION_NAME", Value: m.Name})
	envVars = append(envVars, corev1.EnvVar{Name: "QDROUTERD_CONF", Value: "/etc/qpid-dispatch/qdrouterd.conf.template"})
	envVars = append(envVars, corev1.EnvVar{Name: "POD_COUNT", Value: strconv.Itoa(int(m.Spec.DeploymentPlan.Size))})
	envVars = append(envVars, corev1.EnvVar{Name: "POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "metadata.namespace",
		},
	},
	})
	envVars = append(envVars, corev1.EnvVar{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "status.podIP",
		},
	},
	})
	if m.Spec.DeploymentPlan.Role == v1alpha1.RouterRoleInterior {
		envVars = append(envVars, corev1.EnvVar{Name: "QDROUTERD_AUTO_MESH_DISCOVERY", Value: "QUERY"})
	}
	return envVars
}

func CheckQdrouterdContainer(desired *corev1.Container, actual *corev1.Container) bool {
	if desired.Image != actual.Image {
		return false
	}
	if !reflect.DeepEqual(desired.Env, actual.Env) {
		return false
	}
	if !reflect.DeepEqual(desired.Ports, actual.Ports) {
		return false
	}
	if !reflect.DeepEqual(desired.VolumeMounts, actual.VolumeMounts) {
		return false
	}
	return true
}

func ContainerForQdrouterd(m *v1alpha1.Qdrouterd) corev1.Container {
	var image string
	if m.Spec.DeploymentPlan.Image != "" {
		image = m.Spec.DeploymentPlan.Image
	} else {
		image = os.Getenv("QDROUTERD_IMAGE")
	}
	container := corev1.Container{
		Image: image,
		Name:  m.Name,
		LivenessProbe: &corev1.Probe{
			InitialDelaySeconds: 60,
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Port: intstr.FromInt(constants.HttpLivenessPort),
				},
			},
		},
		Env:   containerEnvVarsForQdrouterd(m),
		Ports: containerPortsForQdrouterd(m),
	}
	volumeMounts := []corev1.VolumeMount{}
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      m.Name,
		MountPath: "/etc/qpid-dispatch/",
	})
	if m.Spec.SslProfiles != nil && len(m.Spec.SslProfiles) > 0 {
		for _, profile := range m.Spec.SslProfiles {
			if len(profile.Credentials) > 0 {
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      profile.Credentials,
					MountPath: "/etc/qpid-dispatch-certs/" + profile.Name + "/" + profile.Credentials,
				})
			}
			if len(profile.CaCert) > 0 && profile.CaCert != profile.Credentials {
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      profile.CaCert,
					MountPath: "/etc/qpid-dispatch-certs/" + profile.Name + "/" + profile.CaCert,
				})
			}

		}
	}
	container.VolumeMounts = volumeMounts
	container.Resources = m.Spec.DeploymentPlan.Resources
	return container
}
