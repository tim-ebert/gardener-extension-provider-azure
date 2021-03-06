// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operation

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	gardenertest "github.com/gardener/gardener/test/integration/framework"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WaitForExtensionCondition waits for the extension to contain the condition type, status and reason
func WaitForExtensionCondition(ctx context.Context, logger *logrus.Logger, seedClient client.Client, groupVersionKind schema.GroupVersionKind, namespacedName types.NamespacedName, conditionType gardencorev1beta1.ConditionType, conditionStatus gardencorev1beta1.ConditionStatus, conditionReason string) error {
	return retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		rawExtension := unstructured.Unstructured{}
		rawExtension.SetGroupVersionKind(groupVersionKind)

		if err := seedClient.Get(ctx, namespacedName, &rawExtension); err != nil {
			logger.Infof("unable to retrieve extension from seed (ns: %s, name: %s, kind %s): %v", namespacedName.Namespace, namespacedName.Name, groupVersionKind.Kind, err)
			return retry.MinorError(fmt.Errorf("unable to retrieve extension from seed (ns: %s, name: %s, kind %s)", namespacedName.Namespace, namespacedName.Name, groupVersionKind.Kind))
		}

		acc, err := extensions.Accessor(rawExtension.DeepCopyObject())
		if err != nil {
			return retry.MinorError(err)
		}

		for _, condition := range acc.GetExtensionStatus().GetConditions() {
			logger.Infof("extension (ns: %s, name: %s, kind %s) has condition: ConditionType: %s, ConditionStatus: %s, ConditionReason: %s))", namespacedName.Namespace, namespacedName.Name, groupVersionKind.Kind, condition.Type, condition.Status, condition.Reason)
			if condition.Type == conditionType && condition.Status == conditionStatus && condition.Reason == conditionReason {
				logger.Infof("found expected condition.")
				return retry.Ok()
			}
		}
		logger.Infof("extension (ns: %s, name: %s, kind %s) does not yet contain expected condition. EXPECTED: (conditionType: %s, conditionStatus: %s, conditionReason: %s))", namespacedName.Namespace, namespacedName.Name, groupVersionKind.Kind, conditionType, conditionStatus, conditionReason)
		return retry.MinorError(fmt.Errorf("extension (ns: %s, name: %s, kind %s) does not yet contain expected condition. EXPECTED: (conditionType: %s, conditionStatus: %s, conditionReason: %s))", namespacedName.Namespace, namespacedName.Name, groupVersionKind.Kind, conditionType, conditionStatus, conditionReason))
	})
}

// ScaleDeployment scales a deployment
func ScaleDeployment(setupContextTimeout time.Duration, client client.Client, desiredReplicas *int32, name, namespace string) (*int32, error) {
	if desiredReplicas == nil {
		return nil, nil
	}

	ctxSetup, cancelCtxSetup := context.WithTimeout(context.Background(), setupContextTimeout)
	defer cancelCtxSetup()

	replicas, err := gardenertest.GetDeploymentReplicas(ctxSetup, client, namespace, name)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve the replica count of the %s deployment: '%v'", name, err)
	}
	if replicas == nil || *replicas == *desiredReplicas {
		return nil, nil
	}
	// scale the deployment
	if err := kubernetes.ScaleDeployment(ctxSetup, client, kutil.Key(namespace, name), *desiredReplicas); err != nil {
		return nil, fmt.Errorf("failed to scale the replica count of the %s deployment: '%v'", name, err)
	}

	// wait until scaled
	if err := gardenertest.WaitUntilDeploymentScaled(ctxSetup, client, namespace, name, *desiredReplicas); err != nil {
		return nil, fmt.Errorf("failed to wait until the %s deployment is scaled: '%v'", name, err)
	}
	return replicas, nil
}

// ScaleGardenerScheduler scales the gardener-scheduler to the desired replicas
func ScaleGardenerResourceManager(setupContextTimeout time.Duration, namespace string, client client.Client, desiredReplicas *int32) (*int32, error) {
	return ScaleDeployment(setupContextTimeout, client, desiredReplicas, "gardener-resource-manager", namespace)
}
