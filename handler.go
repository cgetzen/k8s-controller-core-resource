package main

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Handler interface contains the methods that are required
type Handler interface {
	Init() error
	ObjectCreated(obj interface{})
	// ObjectDeleted(obj interface{})
	// ObjectUpdated(objOld, objNew interface{})
}

// TestHandler is a sample implementation of Handler
type TestHandler struct {
	client kubernetes.Interface
}

// Init handles any handler initialization
func (t *TestHandler) Init() error {
	log.Info("TestHandler.Init")
	return nil
}

// ObjectCreated is called when an object is created
func (t *TestHandler) ObjectCreated(obj interface{}) {
	event := obj.(*core_v1.Event)
	if event.InvolvedObject.Kind == "Pod" {
		if event.Count != 1 {
			return
		}
		switch event.Reason {
		case "Scheduled":
			//
			pod, err := t.client.CoreV1().Pods(event.InvolvedObject.Namespace).Get(event.InvolvedObject.Name, meta_v1.GetOptions{})
			if err != nil {
				return
			}
			creationTime := pod.ObjectMeta.CreationTimestamp
			scheduleDuration := event.CreationTimestamp.Sub(creationTime.Time)
			log.Infof("CHAR Scheduled time for pod %s/%s: %s", pod.Namespace, pod.Name, scheduleDuration)
		// case "Pulling":
		case "Pulled":
			containerName := strings.Split(event.InvolvedObject.FieldPath, "{")[1]
			// If pulling happened, grab pull time, otherwise grab Mount time
			if strings.Contains(event.Message, "Successfully pulled image") {
				opts := meta_v1.ListOptions{
					FieldSelector: fmt.Sprintf("involvedObject.fieldPath=%s,involvedObject.name=%s,reason=Pulling",
						event.InvolvedObject.FieldPath,
						event.InvolvedObject.Name,
					),
				}
				pullingEvents, err := t.client.CoreV1().Events(event.InvolvedObject.Namespace).List(opts)
				if err != nil {
					return
				}
				if len(pullingEvents.Items) != 1 {
					return
				}
				pullingEvent := pullingEvents.Items[0]
				if pullingEvent.Count != 1 {
					return
				}
				pullingTime := pullingEvent.CreationTimestamp
				pullingDuration := event.CreationTimestamp.Sub(pullingTime.Time)
				log.Infof("CHAR Pulled time for pod %s/%s{%s: %s",
					event.InvolvedObject.Namespace,
					event.InvolvedObject.Name,
					containerName,
					pullingDuration,
				)
			} else {
				// Only send event when this is the first container
				pod, err := t.client.CoreV1().Pods(event.InvolvedObject.Namespace).Get(event.InvolvedObject.Name, meta_v1.GetOptions{})
				if err != nil {
					return
				}
				firstContainer := pod.Spec.Containers[0].Name
				if strings.Contains(containerName, firstContainer) {
					// Grab Latest Mount Time
					opts := meta_v1.ListOptions{
						FieldSelector: fmt.Sprintf("involvedObject.name=%s,reason=SuccessfulMountVolume",
							event.InvolvedObject.Name,
						),
					}
					mountingEvents, err := t.client.CoreV1().Events(event.InvolvedObject.Namespace).List(opts)
					if err != nil || len(mountingEvents.Items) == 0 {
						return
					}

					latestMountTime := mountingEvents.Items[0].CreationTimestamp
					for _, mountEvent := range mountingEvents.Items {
						if mountEvent.CreationTimestamp.After(latestMountTime.Time) {
							latestMountTime = mountEvent.CreationTimestamp
						}
					}
					opts = meta_v1.ListOptions{
						FieldSelector: fmt.Sprintf("involvedObject.name=%s,reason=Scheduled",
							event.InvolvedObject.Name,
						),
					}
					scheduledEvents, err := t.client.CoreV1().Events(event.InvolvedObject.Namespace).List(opts)
					if err != nil || len(scheduledEvents.Items) != 1 {
						return
					}
					scheduledTime := scheduledEvents.Items[0].CreationTimestamp
					mountingDuration := latestMountTime.Sub(scheduledTime.Time)
					log.Infof("CHAR Mount time for pod %s/%s: %s",
						event.Namespace,
						event.InvolvedObject.Name,
						mountingDuration,
					)
				}

			}
			// case "Created":
			// case "Started":
		}

		// log.Info("TestHandler.ObjectCreated")
		// log.Infof("    Name: %s", event.InvolvedObject.Name)
		// log.Infof("    Reason: %s", event.Reason)
		// log.Infof("    Count: %d", event.Count)
		// log.Infof("    Message: %s", event.Message)
	}
}

// // ObjectDeleted is called when an object is deleted
// func (t *TestHandler) ObjectDeleted(obj interface{}) {
// 	log.Info("TestHandler.ObjectDeleted")
// }

// // ObjectUpdated is called when an object is updated
// func (t *TestHandler) ObjectUpdated(objOld, objNew interface{}) {
// 	log.Info("TestHandler.ObjectUpdated")
// }
