package req

type OpenMineCellRequest struct {
	X int `json:"x" binding:"min=0,max=127"`
	Y int `json:"y" binding:"min=0,max=127"`
}

type FlagMineCellRequest struct {
	X       int  `json:"x" binding:"min=0,max=127"`
	Y       int  `json:"y" binding:"min=0,max=127"`
	Flagged bool `json:"flagged"`
}
