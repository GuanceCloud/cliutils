package point

func (p *Point) clear() {
	if p.pt != nil {
		p.pt.Name = ""
		p.pt.Fields = p.pt.Fields[:0]
		p.pt.Time = 0
		p.pt.Warns = p.pt.Warns[:0]
		p.pt.Debugs = p.pt.Debugs[:0]
		p.pt.Fields = p.pt.Fields[:0]
	}
}

func (p *Point) Reset() {
	p.flags = 0
	p.clear()
}
