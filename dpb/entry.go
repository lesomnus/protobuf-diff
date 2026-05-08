package dpb

import (
	"github.com/lesomnus/protobuf-diff/ref"
	"github.com/lesomnus/protobuf-diff/target"
)

func (x *Entry) AppendTargets(targets target.Targets) {
	bs := x.GetTargets()
	bs = append(bs, targets.Value()...)
	x.SetTargets(bs)
}

func (x *Entry) CopiedFrom(r ref.Ref) {
	x.SetCopied(r.Value())
}

func (x *Entry) ScatteredFrom(r ref.Ref) {
	x.SetScattered(r.Value())
}

func (x *Entry) SwappedWith(r ref.Ref) {
	x.SetSwapped(r.Value())
}

func (x *Entry) Path() Path {
	return Path{x.GetPath()}
}
