package commander

import (
	"context"
	"path/filepath"
	"time"
)

func (p *pane) requestDirectorySize() {
	e := p.current()
	// ponytail: archive folders keep their existing status; add archive-aware sizing when requested.
	if !e.dir || e.name == ".." || p.inArchive() {
		return
	}
	if !e.sizeRequested {
		p.entries[p.cursor].size = 0
	}
	p.entries[p.cursor].sizeRequested = true
	p.scanDirectorySize()
}

// One worker and one completion per pane; requests live in the existing listing.
func (p *pane) scanDirectorySize() {
	if p.sizeBusy {
		return
	}
	for _, e := range p.entries {
		if !e.sizeRequested || e.sizeKnown || e.sizeError != "" {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		p.sizeCancel, p.sizeBusy = cancel, true
		generation, name := p.iconGeneration, e.name
		path, measure := filepath.Join(p.path, name), p.measureSize
		go func() {
			report := directorySizeProgress(time.Now, latestUpdate(post, func(value [2]int64) {
				if ctx.Err() != nil || p.iconGeneration != generation {
					return
				}
				for i := range p.entries {
					if p.entries[i].name == name && !p.entries[i].sizeKnown && p.entries[i].sizeError == "" {
						p.entries[i].size, p.entries[i].sizeFiles = value[0], value[1]
						p.updateStatus()
						break
					}
				}
			}))
			size, files, err := measure(ctx, []string{path}, report)
			post(func() {
				p.sizeBusy = false
				if ctx.Err() == nil && p.iconGeneration == generation {
					for i := range p.entries {
						if p.entries[i].name == name {
							if err != nil {
								p.entries[i].sizeError = err.Error()
							} else {
								p.entries[i].size, p.entries[i].sizeKnown = size, true
								p.entries[i].sizeFiles = files
							}
							break
						}
					}
					p.updateStatus()
				}
				cancel()
				p.scanDirectorySize()
			})
		}()
		return
	}
}

// Throttle scan reports, then coalesce queued UI work; completion stays separate.
func directorySizeProgress(now func() time.Time, update func([2]int64)) func(int64, int64) {
	last := now()
	return func(bytes, files int64) {
		if current := now(); current.Sub(last) >= time.Second {
			last = current
			update([2]int64{bytes, files})
		}
	}
}
