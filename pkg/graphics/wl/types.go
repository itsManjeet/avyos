package wl

type Dispatcher interface {
	Dispatch(uint32, int, []byte)
}

type Proxy interface {
	Context() *Context
	SetContext(*Context)
	Id() uint32
	SetId(uint32)
}

type BaseProxy struct {
	ctxt *Context
	id   uint32
}

func (p *BaseProxy) Id() uint32 {
	return p.id
}

func (p *BaseProxy) SetId(id uint32) {
	p.id = id
}

func (p *BaseProxy) Context() *Context {
	return p.ctxt
}

func (p *BaseProxy) SetContext(ctxt *Context) {
	p.ctxt = ctxt
}
