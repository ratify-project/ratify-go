/*
Copyright The Ratify Authors.
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

package ratify

import "sync"


// task is a struct that represents a task that verifies an artifact by
// the executor.
type task struct {
	// artifact is the digested reference of the referrer artifact that will be
	// verified.
	artifact string

	// subjectReport is the report of the subject artifact.
	subjectReport *threadSafeReport
}

// threadSafeReport is a struct that wraps a ValidationReport and provides
// thread-safe access to it.
type threadSafeReport struct {
	report *ValidationReport
	lock   sync.Mutex
}

func newSubjectReport(subjectReport *ValidationReport) *threadSafeReport {
	return &threadSafeReport{
		report: subjectReport,
		lock:   sync.Mutex{},
	}
}

func newTask(artifact string, subjectReport *ValidationReport) *task {
	var report *threadSafeReport
	if subjectReport != nil {
		report = newSubjectReport(subjectReport)
	}
	return &task{
		artifact:      artifact,
		subjectReport: report,
	}
}

// addArtifactReports adds artifact reports to the subject report.
func (p *threadSafeReport) addArtifactReports(reports []*ValidationReport) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.report == nil {
		p.report = &ValidationReport{
			ArtifactReports: make([]*ValidationReport, 0),
		}
	}

	p.report.ArtifactReports = append(p.report.ArtifactReports, reports...)
}
