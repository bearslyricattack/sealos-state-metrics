//nolint:testpackage // Tests exercise unexported collector state and helpers.
package namespace

import (
	"reflect"
	"testing"
	"time"

	collectorpkg "github.com/labring/sealos-state-metrics/pkg/collector"
	"github.com/labring/sealos-state-metrics/pkg/collector/base"
	configpkg "github.com/labring/sealos-state-metrics/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

func TestNormalizeLabels(t *testing.T) {
	got := normalizeLabels([]string{" team ", "team", "", "owner"})

	want := []string{"team", "owner"}

	if len(got) != len(want) {
		t.Fatalf("normalizeLabels() = %#v, want %#v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeLabels()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMakeNamespaceSet(t *testing.T) {
	set := makeNamespaceSet([]string{"kube-system", "default", "default"})

	if _, ok := set["kube-system"]; !ok {
		t.Fatal("makeNamespaceSet() did not include kube-system")
	}

	if len(set) != 2 {
		t.Fatalf("makeNamespaceSet() length = %d, want 2", len(set))
	}
}

func TestValidateRequiredLabels(t *testing.T) {
	if err := validateRequiredLabels([]string{"sealos.io/account", "owner"}); err != nil {
		t.Fatalf("validateRequiredLabels() unexpected error = %v", err)
	}

	if err := validateRequiredLabels([]string{"bad label"}); err == nil {
		t.Fatal("validateRequiredLabels() error = nil, want invalid label error")
	}
}

func TestMissingLabels(t *testing.T) {
	c := &Collector{config: &Config{RequiredLabels: []string{"team", "owner"}}}

	missing := c.missingLabels(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "demo",
		Labels: map[string]string{"team": "platform", "owner": ""},
	}})
	if len(missing) != 0 {
		t.Fatalf("missingLabels() = %#v, want no missing labels", missing)
	}

	missing = c.missingLabels(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "demo",
		Labels: map[string]string{"team": "platform"},
	}})
	if len(missing) != 1 || missing[0] != "owner" {
		t.Fatalf("missingLabels() = %#v, want [owner]", missing)
	}
}

func TestCollectEmitsCountAndNamespaceDetails(t *testing.T) {
	c := &Collector{
		BaseCollector: base.NewBaseCollector("namespace", log.NewEntry(log.New())),
		config:        &Config{RequiredLabels: []string{"team", "owner"}},
		namespaces: map[string]*corev1.Namespace{
			"complete": {
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"team":  "platform",
						"owner": "alice",
					},
				},
			},
			"partial": {
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"team": "platform"},
				},
			},
			"empty": {ObjectMeta: metav1.ObjectMeta{}},
		},
	}
	c.initMetrics("sealos")

	ch := make(chan prometheus.Metric, 10)
	c.collect(ch)
	close(ch)

	var count float64

	details := make(map[string]string)

	for metric := range ch {
		pb := &dto.Metric{}
		if err := metric.Write(pb); err != nil {
			t.Fatalf("metric.Write() error = %v", err)
		}

		if pb.GetCounter() != nil {
			continue
		}

		if pb.GetGauge() != nil && len(pb.GetLabel()) == 0 {
			count = pb.GetGauge().GetValue()
		} else {
			labels := make(map[string]string, len(pb.GetLabel()))
			for _, label := range pb.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}

			details[labels["namespace"]] = labels["missing_labels"]
		}
	}

	if count != 2 {
		t.Fatalf("count = %v, want 2", count)
	}

	if len(details) != 2 {
		t.Fatalf("details = %d, want 2", len(details))
	}

	if got := details["partial"]; got != "owner" {
		t.Fatalf("partial missing_labels = %q, want %q", got, "owner")
	}

	if got := details["empty"]; got != "team,owner" {
		t.Fatalf("empty missing_labels = %q, want %q", got, "team,owner")
	}

	if got := details["complete"]; got != "" {
		t.Fatalf("complete unexpectedly emitted details: %#v", details)
	}
}

func TestNamespaceDeleteRemovesMetrics(t *testing.T) {
	c := &Collector{
		BaseCollector: base.NewBaseCollector("namespace", log.NewEntry(log.New())),
		config:        &Config{RequiredLabels: []string{"team"}},
		namespaces:    make(map[string]*corev1.Namespace),
		logger:        log.NewEntry(log.New()),
	}
	c.initMetrics("sealos")

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "temporary"}}
	c.handleNamespaceAdd(namespace)

	if got := collectCount(t, c); got != 1 {
		t.Fatalf("count after add = %v, want 1", got)
	}

	c.handleNamespaceDelete(cache.DeletedFinalStateUnknown{Key: "temporary", Obj: namespace})

	if got := collectCount(t, c); got != 0 {
		t.Fatalf("count after delete = %v, want 0", got)
	}

	if got := collectCreatedTotal(t, c); got != 1 {
		t.Fatalf("created total after delete = %v, want 1", got)
	}
}

func TestNamespaceCreationCounterOnlyCountsDangerousAdds(t *testing.T) {
	c := &Collector{
		BaseCollector: base.NewBaseCollector("namespace", log.NewEntry(log.New())),
		config:        &Config{RequiredLabels: []string{"team"}},
		namespaces:    make(map[string]*corev1.Namespace),
		logger:        log.NewEntry(log.New()),
	}
	c.initMetrics("sealos")

	dangerous := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dangerous"}}
	c.handleNamespaceAdd(dangerous)

	if got := collectCreatedTotal(t, c); got != 1 {
		t.Fatalf("created total after dangerous add = %v, want 1", got)
	}

	// Updates do not represent a new namespace creation, even if labels remain
	// missing or are removed later.
	c.handleNamespaceUpdate(dangerous, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "dangerous", Labels: map[string]string{"owner": "alice"},
	}})

	if got := collectCreatedTotal(t, c); got != 1 {
		t.Fatalf("created total after update = %v, want 1", got)
	}

	c.handleNamespaceAdd(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "complete", Labels: map[string]string{"team": "platform"},
	}})

	if got := collectCreatedTotal(t, c); got != 1 {
		t.Fatalf("created total after complete add = %v, want 1", got)
	}
}

func TestNamespaceLabelChangeCounterTracksRequiredLabelUpdates(t *testing.T) {
	c := &Collector{
		BaseCollector: base.NewBaseCollector("namespace", log.NewEntry(log.New())),
		config:        &Config{RequiredLabels: []string{"team", "owner"}},
		namespaces:    make(map[string]*corev1.Namespace),
		logger:        log.NewEntry(log.New()),
	}
	c.initMetrics("sealos")

	oldNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "demo",
		Labels: map[string]string{"team": "platform", "unrelated": "before"},
	}}
	c.handleNamespaceAdd(oldNamespace)

	if got := collectChangedTotal(t, c); got != 0 {
		t.Fatalf("changed total after add = %v, want 0", got)
	}

	// A required label value change is one event, even when the namespace is
	// still missing another required label.
	newNamespace := oldNamespace.DeepCopy()
	newNamespace.Labels["team"] = "engineering"
	c.handleNamespaceUpdate(oldNamespace, newNamespace)

	if got := collectChangedTotal(t, c); got != 1 {
		t.Fatalf("changed total after required label value change = %v, want 1", got)
	}

	// Changes to an unmonitored label do not count.
	oldNamespace = newNamespace
	newNamespace = oldNamespace.DeepCopy()
	newNamespace.Labels["unrelated"] = "after"
	c.handleNamespaceUpdate(oldNamespace, newNamespace)

	if got := collectChangedTotal(t, c); got != 1 {
		t.Fatalf("changed total after unrelated label change = %v, want 1", got)
	}

	// Adding and removing required labels are also changes.
	oldNamespace = newNamespace
	newNamespace = oldNamespace.DeepCopy()
	newNamespace.Labels["owner"] = "alice"
	c.handleNamespaceUpdate(oldNamespace, newNamespace)

	if got := collectChangedTotal(t, c); got != 2 {
		t.Fatalf("changed total after required label add = %v, want 2", got)
	}

	oldNamespace = newNamespace
	newNamespace = oldNamespace.DeepCopy()
	delete(newNamespace.Labels, "team")
	c.handleNamespaceUpdate(oldNamespace, newNamespace)

	if got := collectChangedTotal(t, c); got != 3 {
		t.Fatalf("changed total after required label removal = %v, want 3", got)
	}

	// Metadata-only and no-op updates do not count.
	oldNamespace = newNamespace
	newNamespace = oldNamespace.DeepCopy()
	newNamespace.ResourceVersion = "2"
	c.handleNamespaceUpdate(oldNamespace, newNamespace)

	if got := collectChangedTotal(t, c); got != 3 {
		t.Fatalf("changed total after metadata-only update = %v, want 3", got)
	}

	c.handleNamespaceUpdate(newNamespace, newNamespace.DeepCopy())

	if got := collectChangedTotal(t, c); got != 3 {
		t.Fatalf("changed total after no-op update = %v, want 3", got)
	}

	c.handleNamespaceDelete(newNamespace)

	if got := collectChangedTotal(t, c); got != 3 {
		t.Fatalf("changed total after delete = %v, want 3", got)
	}
}

func TestNamespaceLabelChangeCounterSkipsWhitelistedNamespaces(t *testing.T) {
	c := &Collector{
		BaseCollector: base.NewBaseCollector("namespace", log.NewEntry(log.New())),
		config:        &Config{RequiredLabels: []string{"team"}},
		whitelist:     makeNamespaceSet([]string{"ignored"}),
		namespaces:    make(map[string]*corev1.Namespace),
		logger:        log.NewEntry(log.New()),
	}
	c.initMetrics("sealos")

	oldNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "ignored",
		Labels: map[string]string{"team": "platform"},
	}}
	newNamespace := oldNamespace.DeepCopy()
	newNamespace.Labels["team"] = "engineering"
	c.handleNamespaceUpdate(oldNamespace, newNamespace)

	if got := collectChangedTotal(t, c); got != 0 {
		t.Fatalf("changed total after whitelisted update = %v, want 0", got)
	}

	if got := collectNamespaceCounterValues(t, c, c.missingLabelsCreatedByNamespaceTotal); len(got) != 0 {
		t.Fatalf("created-by-namespace metrics after whitelisted update = %#v, want none", got)
	}

	if got := collectNamespaceCounterValues(t, c, c.missingLabelsUpdatedByNamespaceTotal); len(got) != 0 {
		t.Fatalf("updated-by-namespace metrics after whitelisted update = %#v, want none", got)
	}
}

func TestNamespaceEventCountersTrackEventsByNamespace(t *testing.T) {
	c := &Collector{
		BaseCollector: base.NewBaseCollector("namespace", log.NewEntry(log.New())),
		config:        &Config{RequiredLabels: []string{"team", "owner"}},
		namespaces:    make(map[string]*corev1.Namespace),
		logger:        log.NewEntry(log.New()),
	}
	c.initMetrics("sealos")

	dangerous := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dangerous"}}
	c.handleNamespaceAdd(dangerous)

	complete := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "complete",
		Labels: map[string]string{"team": "platform", "owner": "alice"},
	}}
	c.handleNamespaceAdd(complete)

	updated := dangerous.DeepCopy()
	updated.Labels = map[string]string{"team": "platform"}
	c.handleNamespaceUpdate(dangerous, updated)

	// Non-required label updates do not create an update event sample.
	unchangedRequiredLabels := updated.DeepCopy()
	unchangedRequiredLabels.Labels["unrelated"] = "value"
	c.handleNamespaceUpdate(updated, unchangedRequiredLabels)

	secondDangerous := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "second-dangerous"}}
	c.handleNamespaceAdd(secondDangerous)
	c.handleNamespaceDelete(dangerous)

	created := collectNamespaceCounterValues(t, c, c.missingLabelsCreatedByNamespaceTotal)
	if want := map[string]float64{"dangerous": 1, "second-dangerous": 1}; !reflect.DeepEqual(created, want) {
		t.Fatalf("created-by-namespace metrics = %#v, want %#v", created, want)
	}

	updatedByNamespace := collectNamespaceCounterValues(t, c, c.missingLabelsUpdatedByNamespaceTotal)
	if want := map[string]float64{"dangerous": 1, "second-dangerous": 0}; !reflect.DeepEqual(updatedByNamespace, want) {
		t.Fatalf("updated-by-namespace metrics = %#v, want %#v", updatedByNamespace, want)
	}
}

func TestNamespaceWhitelistExcludesAllMetrics(t *testing.T) {
	c := &Collector{
		BaseCollector: base.NewBaseCollector("namespace", log.NewEntry(log.New())),
		config:        &Config{RequiredLabels: []string{"team"}},
		whitelist:     makeNamespaceSet([]string{"ignored"}),
		namespaces:    make(map[string]*corev1.Namespace),
		logger:        log.NewEntry(log.New()),
	}
	c.initMetrics("sealos")

	c.handleNamespaceAdd(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ignored"}})

	if got := collectCount(t, c); got != 0 {
		t.Fatalf("count after whitelisted add = %v, want 0", got)
	}

	if got := collectCreatedTotal(t, c); got != 0 {
		t.Fatalf("created total after whitelisted add = %v, want 0", got)
	}

	// A pre-existing entry must also be filtered when collecting. This keeps
	// the whitelist behavior correct if configuration is assembled manually.
	c.namespaces["ignored"] = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ignored"}}

	if got := collectCount(t, c); got != 0 {
		t.Fatalf("count with whitelisted cache entry = %v, want 0", got)
	}

	c.handleNamespaceAdd(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tracked"}})

	if got := collectCount(t, c); got != 1 {
		t.Fatalf("count after non-whitelisted add = %v, want 1", got)
	}

	if got := collectCreatedTotal(t, c); got != 1 {
		t.Fatalf("created total after non-whitelisted add = %v, want 1", got)
	}
}

func TestNewCollectorLoadsAndNormalizesWhitelist(t *testing.T) {
	created, err := NewCollector(&collectorpkg.FactoryContext{
		ConfigLoader: configpkg.NewModuleConfigLoader([]byte(`
collectors:
  namespace:
    requiredLabels: [team]
    whitelist: [" kube-system ", kube-system, default]
`)),
		MetricsNamespace:     "sealos",
		InformerResyncPeriod: time.Minute,
		Logger:               log.NewEntry(log.New()),
		ClientProvider:       staticClientProvider{client: fake.NewClientset()},
	})
	if err != nil {
		t.Fatalf("NewCollector() error = %v", err)
	}

	c, ok := created.(*Collector)
	if !ok {
		t.Fatalf("NewCollector() returned %T, want *Collector", created)
	}

	if len(c.config.Whitelist) != 2 {
		t.Fatalf("normalized whitelist = %#v, want two entries", c.config.Whitelist)
	}

	if !c.isWhitelisted("kube-system") || !c.isWhitelisted("default") {
		t.Fatalf("whitelist set = %#v, want kube-system and default", c.whitelist)
	}
}

func TestRestartDropsNamespacesDeletedWhileStopped(t *testing.T) {
	client := fake.NewClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "temporary"},
	})

	created, err := NewCollector(&collectorpkg.FactoryContext{
		ConfigLoader: configpkg.NewModuleConfigLoader([]byte(`
collectors:
  namespace:
    requiredLabels: [team]
`)),
		MetricsNamespace:     "sealos",
		InformerResyncPeriod: time.Minute,
		Logger:               log.NewEntry(log.New()),
		ClientProvider:       staticClientProvider{client: client},
	})
	if err != nil {
		t.Fatalf("NewCollector() error = %v", err)
	}

	c, ok := created.(*Collector)
	if !ok {
		t.Fatalf("NewCollector() returned %T, want *Collector", created)
	}

	ctx := t.Context()

	if err := c.Start(ctx); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}

	if got := collectCount(t, c); got != 1 {
		t.Fatalf("count before stop = %v, want 1", got)
	}

	if err := c.Stop(); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}

	deleteErr := client.CoreV1().Namespaces().Delete(ctx, "temporary", metav1.DeleteOptions{})
	if deleteErr != nil && !errors.IsNotFound(deleteErr) {
		t.Fatalf("Delete() error = %v", deleteErr)
	}

	if err := c.Start(ctx); err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	defer func() {
		if err := c.Stop(); err != nil {
			t.Errorf("second Stop() error = %v", err)
		}
	}()

	if got := collectCount(t, c); got != 0 {
		t.Fatalf("count after restart = %v, want 0", got)
	}
}

type staticClientProvider struct {
	client kubernetes.Interface
}

func (p staticClientProvider) GetRestConfig() (*rest.Config, error) {
	return &rest.Config{}, nil
}

func (p staticClientProvider) GetClient() (kubernetes.Interface, error) {
	return p.client, nil
}

func collectCount(t *testing.T, c *Collector) float64 {
	t.Helper()

	ch := make(chan prometheus.Metric, 32)
	c.collect(ch)
	close(ch)

	values := make([]float64, 0, 1)
	for metric := range ch {
		pb := &dto.Metric{}
		if err := metric.Write(pb); err != nil {
			t.Fatalf("metric.Write() error = %v", err)
		}

		if len(pb.GetLabel()) == 0 && pb.GetGauge() != nil {
			values = append(values, pb.GetGauge().GetValue())
		}
	}

	if len(values) != 1 {
		t.Fatalf("count metrics = %d, want 1", len(values))
	}

	return values[0]
}

func collectCreatedTotal(t *testing.T, c *Collector) float64 {
	return collectCounterValue(t, c, c.missingLabelsCreatedTotal, "created total")
}

func collectChangedTotal(t *testing.T, c *Collector) float64 {
	return collectCounterValue(t, c, c.missingLabelsChangedTotal, "changed total")
}

func collectNamespaceCounterValues(t *testing.T, c *Collector, target *prometheus.Desc) map[string]float64 {
	t.Helper()

	ch := make(chan prometheus.Metric, 32)
	c.collect(ch)
	close(ch)

	values := make(map[string]float64)
	for metric := range ch {
		if metric.Desc() != target {
			continue
		}

		pb := &dto.Metric{}
		if err := metric.Write(pb); err != nil {
			t.Fatalf("metric.Write() error = %v", err)
		}

		if pb.GetCounter() == nil || len(pb.GetLabel()) != 1 || pb.GetLabel()[0].GetName() != "namespace" {
			t.Fatalf("unexpected namespace counter metric = %#v", pb)
		}

		values[pb.GetLabel()[0].GetValue()] = pb.GetCounter().GetValue()
	}

	return values
}

func collectCounterValue(t *testing.T, c *Collector, target *prometheus.Desc, name string) float64 {
	t.Helper()

	ch := make(chan prometheus.Metric, 32)
	c.collect(ch)
	close(ch)

	values := make([]float64, 0, 1)
	for metric := range ch {
		pb := &dto.Metric{}
		if err := metric.Write(pb); err != nil {
			t.Fatalf("metric.Write() error = %v", err)
		}

		if metric.Desc() == target && len(pb.GetLabel()) == 0 && pb.GetCounter() != nil {
			values = append(values, pb.GetCounter().GetValue())
		}
	}

	if len(values) != 1 {
		t.Fatalf("%s metrics = %d, want 1", name, len(values))
	}

	return values[0]
}
