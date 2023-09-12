/*
Copyright 2023 SAP.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"

	"k8s.io/api/core/v1"
)

// getAnnotationBoolean is a utility function that checks if an annotation is set to true.
func getAnnotationBoolean(key string, svc *v1.Service, _default bool) bool {
	if svc.GetAnnotations() == nil {
		return _default
	}
	v, ok := svc.Annotations[key]
	if !ok {
		return _default
	}
	return v == "true"
}

// getAnnotationString is a utility function that returns the value of an annotation.
func getAnnotationString(key string, svc *v1.Service) (string, bool) {
	if svc.GetAnnotations() == nil {
		return "", false
	}
	v, ok := svc.Annotations[key]
	return v, ok
}

// makeAnnotation is a utility function that constructs the annotation for a given prefix and annotation key.
func makeAnnotation(prefix, annotationKey string) string {
	return fmt.Sprintf("%s/%s", prefix, annotationKey)
}
