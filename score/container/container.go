package container

import (
	"strings"

	"github.com/zegl/kube-score/config"
	"github.com/zegl/kube-score/score/checks"
	"github.com/zegl/kube-score/scorecard"
	corev1 "k8s.io/api/core/v1"
)

func Register(allChecks *checks.Checks, cnf config.Configuration) {
	allChecks.RegisterPodCheck("Container Resources", `Makes sure that all pods have resource limits and requests set. The --ignore-container-cpu-limit flag can be used to disable the requirement of having a CPU limit`, containerResources(!cnf.IgnoreContainerCpuLimitRequirement))
	allChecks.RegisterPodCheck("Container Image Tag", `Makes sure that a explicit non-latest tag is used`, containerImageTag)
	allChecks.RegisterPodCheck("Container Image Pull Policy", `Makes sure that the pullPolicy is set to Always. This makes sure that imagePullSecrets are always validated.`, containerImagePullPolicy)
}

// containerResources makes sure that the container has resource requests and limits set
// The check for a CPU limit requirement can be enabled via the requireCPULimit flag parameter
func containerResources(requireCPULimit bool) func(corev1.PodTemplateSpec, string) scorecard.TestScore {
	return func(podTemplate corev1.PodTemplateSpec, kind string) (score scorecard.TestScore) {
		pod := podTemplate.Spec

		allContainers := pod.InitContainers
		allContainers = append(allContainers, pod.Containers...)

		hasMissingLimit := false
		hasMissingRequest := false

		for _, container := range allContainers {
			if container.Resources.Limits.Cpu().IsZero() && requireCPULimit {
				score.AddComment(container.Name, "CPU limit is not set", "Resource limits are recommended to avoid resource DDOS. Set resources.limits.cpu")
				hasMissingLimit = true
			}
			if container.Resources.Limits.Memory().IsZero() {
				score.AddComment(container.Name, "Memory limit is not set", "Resource limits are recommended to avoid resource DDOS. Set resources.limits.memory")
				hasMissingLimit = true
			}
			if container.Resources.Requests.Cpu().IsZero() {
				score.AddComment(container.Name, "CPU request is not set", "Resource requests are recommended to make sure that the application can start and run without crashing. Set resources.requests.cpu")
				hasMissingRequest = true
			}
			if container.Resources.Requests.Memory().IsZero() {
				score.AddComment(container.Name, "Memory request is not set", "Resource requests are recommended to make sure that the application can start and run without crashing. Set resources.requests.memory")
				hasMissingRequest = true
			}
		}

		if len(allContainers) == 0 {
			score.Grade = scorecard.GradeCritical
			score.AddComment("", "No containers defined", "")
		} else if hasMissingLimit {
			score.Grade = scorecard.GradeCritical
		} else if hasMissingRequest {
			score.Grade = scorecard.GradeWarning
		} else {
			score.Grade = scorecard.GradeAllOK
		}

		return
	}
}

// containerImageTag checks that no container is using the ":latest" tag
func containerImageTag(podTemplate corev1.PodTemplateSpec, king string) (score scorecard.TestScore) {
	pod := podTemplate.Spec

	allContainers := pod.InitContainers
	allContainers = append(allContainers, pod.Containers...)

	hasTagLatest := false

	for _, container := range allContainers {
		tag := containerTag(container.Image)
		if tag == "" || tag == "latest" {
			score.AddComment(container.Name, "Image with latest tag", "Using a fixed tag is recommended to avoid accidental upgrades")
			hasTagLatest = true
		}
	}

	if hasTagLatest {
		score.Grade = scorecard.GradeCritical
	} else {
		score.Grade = scorecard.GradeAllOK
	}

	return
}

// containerImagePullPolicy checks if the containers ImagePullPolicy is set to PullAlways
func containerImagePullPolicy(podTemplate corev1.PodTemplateSpec, kind string) (score scorecard.TestScore) {
	pod := podTemplate.Spec

	allContainers := pod.InitContainers
	allContainers = append(allContainers, pod.Containers...)

	// Default to AllOK
	score.Grade = scorecard.GradeAllOK

	for _, container := range allContainers {
		tag := containerTag(container.Image)

		// If the pull policy is not set, and the tag is either empty or latest
		// kubernetes will default to always pull the image
		if container.ImagePullPolicy == corev1.PullPolicy("") && (tag == "" || tag == "latest") {
			continue
		}

		// No defined pull policy
		if container.ImagePullPolicy != corev1.PullAlways || container.ImagePullPolicy == corev1.PullPolicy("") {
			score.AddComment(container.Name, "ImagePullPolicy is not set to Always", "It's recommended to always set the ImagePullPolicy to Always, to make sure that the imagePullSecrets are always correct, and to always get the image you want.")
			score.Grade = scorecard.GradeCritical
		}
	}

	return
}

// containerTag returns the image tag
// An empty string is returned if the image has no tag
func containerTag(image string) string {
	imageParts := strings.Split(image, ":")
	if len(imageParts) > 1 {
		imageVersion := imageParts[len(imageParts)-1]
		return imageVersion
	}
	return ""
}
