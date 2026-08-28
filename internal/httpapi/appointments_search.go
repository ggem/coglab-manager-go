package httpapi

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ggem/coglab-manager-go/internal/db"
	"github.com/ggem/coglab-manager-go/internal/scheduling"
)

// availabilitySearchInputs is what buildAvailabilitySearch assembles from
// the database, ready to hand to scheduling.SearchAvailability -- the
// boundary between this package's DB/HTTP awareness and that package's
// pure domain logic.
type availabilitySearchInputs struct {
	days              []scheduling.DayAvailability
	roles             []scheduling.RoleCandidatesForSearch
	sitterRole        *scheduling.RoleCandidatesForSearch
	sitterRequirement scheduling.SitterRequirement
}

// buildAvailabilitySearch fetches everything a search over
// [startDate, endDate] needs (capped at maxSearchDays): the experiment's
// training requirements and equipment, each role's trained-member pool,
// and every candidate's availability/busy data for each day in range.
func (s *Server) buildAvailabilitySearch(
	ctx context.Context, experiment db.Experiment, appointment db.Appointment,
	startDate, endDate time.Time,
) (availabilitySearchInputs, error) {
	numDays := max(1, min(maxSearchDays, int(endDate.Sub(startDate).Hours()/24)+1))
	lastDate := startDate.AddDate(0, 0, numDays-1)

	trainingRoles, err := s.queries.ListExperimentTrainingRequirements(ctx, experiment.ID)
	if err != nil {
		return availabilitySearchInputs{}, err
	}

	sitterRoleRow, err := s.queries.GetSitterRoleForLab(ctx, experiment.LabID)
	hasSitterRole := true
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			hasSitterRole = false
		} else {
			return availabilitySearchInputs{}, err
		}
	}

	// The sitter is handled separately below (its requirement depends on
	// sibling_coming, not a flat training requirement), so exclude it here
	// even if a lab happened to also list it as a training requirement.
	var roleRows []db.ExperimentRole
	for _, role := range trainingRoles {
		if hasSitterRole && role.ID == sitterRoleRow.ID {
			continue
		}
		roleRows = append(roleRows, role)
	}

	equipment, err := s.queries.ListExperimentEquipment(ctx, experiment.ID)
	if err != nil {
		return availabilitySearchInputs{}, err
	}

	roleCandidates := make(map[int64][]db.User, len(roleRows))
	for _, role := range roleRows {
		members, err := s.queries.ListLabMemberTrainingsForRole(ctx, role.ID)
		if err != nil {
			return availabilitySearchInputs{}, err
		}
		roleCandidates[role.ID] = members
	}
	var sitterCandidates []db.User
	if hasSitterRole {
		sitterCandidates, err = s.queries.ListLabMemberTrainingsForRole(ctx, sitterRoleRow.ID)
		if err != nil {
			return availabilitySearchInputs{}, err
		}
	}

	// Most-constrained-first (fewest candidates first): the search-pruning
	// ordering internal/scheduling's RoleCandidates doc comment asks
	// callers to provide.
	sortRolesByCandidateCount(roleRows, roleCandidates)

	availableByRole := make(map[int64]map[int64][]scheduling.Slots, len(roleRows))
	for _, role := range roleRows {
		availableByRole[role.ID] = make(map[int64][]scheduling.Slots, len(roleCandidates[role.ID]))
	}
	sitterAvailable := make(map[int64][]scheduling.Slots, len(sitterCandidates))

	rangeAvailability, err := s.fetchRangeAvailability(ctx, experiment.LabID, startDate, lastDate)
	if err != nil {
		return availabilitySearchInputs{}, err
	}

	days := make([]scheduling.DayAvailability, 0, numDays)
	for i := range numDays {
		date := startDate.AddDate(0, 0, i)
		days = append(days, rangeAvailability.dayAvailability(date, equipment))

		for _, role := range roleRows {
			for _, u := range roleCandidates[role.ID] {
				availableByRole[role.ID][u.ID] = append(availableByRole[role.ID][u.ID], rangeAvailability.memberSlots(u.ID, date))
			}
		}
		for _, u := range sitterCandidates {
			sitterAvailable[u.ID] = append(sitterAvailable[u.ID], rangeAvailability.memberSlots(u.ID, date))
		}
	}

	roles := make([]scheduling.RoleCandidatesForSearch, len(roleRows))
	for i, role := range roleRows {
		roles[i] = scheduling.RoleCandidatesForSearch{
			RoleCandidates: scheduling.RoleCandidates{RoleID: role.ID, MemberIDs: userIDs(roleCandidates[role.ID])},
			Available:      availableByRole[role.ID],
		}
	}

	var sitterRoleForSearch *scheduling.RoleCandidatesForSearch
	if hasSitterRole {
		sitterRoleForSearch = &scheduling.RoleCandidatesForSearch{
			RoleCandidates: scheduling.RoleCandidates{RoleID: sitterRoleRow.ID, MemberIDs: userIDs(sitterCandidates)},
			Available:      sitterAvailable,
		}
	}

	sitterRequirement := scheduling.SitterSoft
	switch appointment.SiblingComing {
	case "coming":
		sitterRequirement = scheduling.SitterRequired
	case "not_coming":
		sitterRequirement = scheduling.SitterNotNeeded
	}

	return availabilitySearchInputs{
		days:              days,
		roles:             roles,
		sitterRole:        sitterRoleForSearch,
		sitterRequirement: sitterRequirement,
	}, nil
}

func sortRolesByCandidateCount(roles []db.ExperimentRole, candidates map[int64][]db.User) {
	slices.SortFunc(roles, func(a, b db.ExperimentRole) int {
		return cmp.Compare(len(candidates[a.ID]), len(candidates[b.ID]))
	})
}

func userIDs(users []db.User) []int64 {
	ids := make([]int64, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	return ids
}

// rangeAvailability holds every availability/busy row for a whole search
// range, fetched with one query per data type (rather than one query per
// candidate day) and grouped in Go by the key each row is looked up
// under: weekday for general availability (it's a weekly-recurring
// schedule with no date of its own), date for everything else.
type rangeAvailability struct {
	generalByWeekdayUser map[int16]map[int64][]scheduling.TimeRange
	specificByDateUser   map[string]map[int64][]scheduling.TimeRange
	busyByDateUser       map[string]map[int64][]scheduling.TimeRange
	busyEquipmentByDate  map[string]map[int64][]scheduling.TimeRange
	blockedByDate        map[string][]scheduling.TimeRange
}

func dateKey(d pgtype.Date) string {
	return d.Time.Format(dateLayout)
}

func (r rangeAvailability) memberSlots(userID int64, date time.Time) scheduling.Slots {
	general := r.generalByWeekdayUser[int16(date.Weekday())][userID]
	specific := r.specificByDateUser[date.Format(dateLayout)][userID]
	busy := r.busyByDateUser[date.Format(dateLayout)][userID]
	return scheduling.MemberDaySchedule(general, specific, busy)
}

func (r rangeAvailability) dayAvailability(date time.Time, equipment []db.Equipment) scheduling.DayAvailability {
	key := date.Format(dateLayout)
	busyByEquipment := r.busyEquipmentByDate[key]
	equipmentSlots := make([]scheduling.Slots, len(equipment))
	for i, eq := range equipment {
		equipmentSlots[i] = scheduling.EquipmentDaySchedule(int(eq.Quantity), busyByEquipment[eq.ID])
	}
	return scheduling.DayAvailability{Date: date, EquipmentSlots: equipmentSlots, Blocked: r.blockedByDate[key]}
}

// fetchRangeAvailability issues exactly one query per data type for the
// whole [startDate, lastDate] range, instead of one per candidate day --
// up to maxSearchDays x 5 queries collapsed into 5.
func (s *Server) fetchRangeAvailability(ctx context.Context, labID int64, startDate, lastDate time.Time) (rangeAvailability, error) {
	startPg := pgtype.Date{Time: startDate, Valid: true}
	endPg := pgtype.Date{Time: lastDate, Valid: true}

	general, err := s.queries.ListLabAvailabilityGeneralByLab(ctx, labID)
	if err != nil {
		return rangeAvailability{}, err
	}
	specific, err := s.queries.ListLabAvailabilitySpecificForDateRange(ctx, db.ListLabAvailabilitySpecificForDateRangeParams{LabID: labID, StartDate: startPg, EndDate: endPg})
	if err != nil {
		return rangeAvailability{}, err
	}
	busyPeople, err := s.queries.ListBusyAppointmentExperimentersForDateRange(ctx, db.ListBusyAppointmentExperimentersForDateRangeParams{LabID: labID, StartDate: startPg, EndDate: endPg})
	if err != nil {
		return rangeAvailability{}, err
	}
	busyEquipment, err := s.queries.ListBusyEquipmentForDateRange(ctx, db.ListBusyEquipmentForDateRangeParams{LabID: labID, StartDate: startPg, EndDate: endPg})
	if err != nil {
		return rangeAvailability{}, err
	}
	blockings, err := s.queries.ListScheduleBlockingsForDateRange(ctx, db.ListScheduleBlockingsForDateRangeParams{LabID: labID, StartDate: startPg, EndDate: endPg})
	if err != nil {
		return rangeAvailability{}, err
	}

	r := rangeAvailability{
		generalByWeekdayUser: map[int16]map[int64][]scheduling.TimeRange{},
		specificByDateUser:   map[string]map[int64][]scheduling.TimeRange{},
		busyByDateUser:       map[string]map[int64][]scheduling.TimeRange{},
		busyEquipmentByDate:  map[string]map[int64][]scheduling.TimeRange{},
		blockedByDate:        map[string][]scheduling.TimeRange{},
	}
	for _, g := range general {
		byUser, ok := r.generalByWeekdayUser[g.Weekday]
		if !ok {
			byUser = map[int64][]scheduling.TimeRange{}
			r.generalByWeekdayUser[g.Weekday] = byUser
		}
		byUser[g.UserID] = append(byUser[g.UserID], scheduling.TimeRange{
			Start: pgTimeToDuration(g.StartTime), End: pgTimeToDuration(g.EndTime),
		})
	}
	for _, sp := range specific {
		key := dateKey(sp.Date)
		byUser, ok := r.specificByDateUser[key]
		if !ok {
			byUser = map[int64][]scheduling.TimeRange{}
			r.specificByDateUser[key] = byUser
		}
		byUser[sp.UserID] = append(byUser[sp.UserID], scheduling.TimeRange{
			Start: pgTimeToDuration(sp.StartTime), End: pgTimeToDuration(sp.EndTime),
		})
	}
	for _, b := range busyPeople {
		key := dateKey(b.ScheduleDate)
		byUser, ok := r.busyByDateUser[key]
		if !ok {
			byUser = map[int64][]scheduling.TimeRange{}
			r.busyByDateUser[key] = byUser
		}
		byUser[b.UserID] = append(byUser[b.UserID], scheduling.TimeRange{
			Start: pgTimeToDuration(b.ScheduleTimeStart), End: pgTimeToDuration(b.ScheduleTimeEnd),
		})
	}
	for _, be := range busyEquipment {
		key := dateKey(be.ScheduleDate)
		byEquipment, ok := r.busyEquipmentByDate[key]
		if !ok {
			byEquipment = map[int64][]scheduling.TimeRange{}
			r.busyEquipmentByDate[key] = byEquipment
		}
		byEquipment[be.EquipmentID] = append(byEquipment[be.EquipmentID], scheduling.TimeRange{
			Start: pgTimeToDuration(be.ScheduleTimeStart), End: pgTimeToDuration(be.ScheduleTimeEnd),
		})
	}
	for _, b := range blockings {
		key := dateKey(b.Date)
		r.blockedByDate[key] = append(r.blockedByDate[key], scheduling.TimeRange{
			Start: pgTimeToDuration(b.StartTime), End: pgTimeToDuration(b.EndTime),
		})
	}

	return r, nil
}
