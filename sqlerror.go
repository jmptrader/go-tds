package gotds

type SQLError struct {
	//The error number (numbers less than 20001 are reserved by Microsoft SQL Server).
	Number int
	// The error state, used as a modifier to the error number.
	State int
	// Class a.k.a. severity determines the severity of the error.
	// Values below 10 indicate informational messages
	Class int
	// The error message itself
	Text string
	// The name of the server
	Server string
	// The name of the procedure that caused the error
	Procedure string
	// The line-number at which the error occured. 1-based
	// 0 means not applicable.
	Line int
}

func (e SQLError) Error() string {

	return e.Text
}
