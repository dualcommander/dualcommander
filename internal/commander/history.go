package commander

type folderLocation struct {
	path, selected string
}

func (p *pane) travelHistory(step int) {
	if p.ui.passwordWindow != nil {
		return
	}
	index := p.historyIndex + step
	if index < 0 || index >= len(p.history) {
		return
	}
	target := p.history[index]
	p.navigateHistory(target.path, target.selected, index)
	p.ui.activate(p)
}
