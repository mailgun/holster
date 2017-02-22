package holster

import "sync"

type WaitGroup struct {
	wg    sync.WaitGroup
	mutex sync.Mutex
	errs  []error
}

// Run a routine and collect errors if any
//func (wg *WaitGroup) Run(callBack func() error) {
func (wg *WaitGroup) Run(callBack func(interface{}) error, data interface{}) {
	wg.wg.Add(1)
	go func() {
		err := callBack(data)
		if err == nil {
			return
		}
		wg.mutex.Lock()
		wg.errs = append(wg.errs, err)
		wg.wg.Done()
		wg.mutex.Unlock()
	}()
}

// Run a routine in a loop continuously, if the callBack return false the loop is broken
func (wg *WaitGroup) Loop(callBack func() bool) {
	wg.wg.Add(1)
	go func() {
		for {
			if !callBack() {
				wg.wg.Done()
				break
			}
		}
	}()
}

// Wait for all the routines to complete and return any errors collected
func (wg *WaitGroup) Wait() []error {
	wg.wg.Wait()
	if len(wg.errs) == 0 {
		return nil
	}
	return wg.errs
}
