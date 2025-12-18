package main

type Init struct {
	su *Supervisor
}

func (s *Init) List(filter string) ([]string, error) {
	resp := make([]string, len(s.su.services))
	for i, v := range s.su.services {
		resp[i] = v.Name
	}
	return resp, nil
}

func (s *Init) Status(name string) (string, error) {
	sv := su.get(name)
	switch sv.State {
	case NotStarted:
		return "not-stated", nil
	case Started:
		return "started", nil
	case Running:
		return "running", nil
	case Finished:
		return "finished", nil
	case Failed:
		return "failed", nil
	}
	return "unknown", nil
}
