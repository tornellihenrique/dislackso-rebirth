package domain

// Policy holds every authorization rule, so services never compare roles
// themselves. Guild rules join it when guilds exist.
type Policy struct{}

func (Policy) ManageInstance(a Actor) error {
	if !a.InstanceAdmin {
		return Forbidden(ReasonNotInstanceAdmin)
	}
	return nil
}
