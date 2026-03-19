ObjC.import('stdlib');

function findCalendar(app, name) {
  var matches = app.calendars.whose({name: name})();
  if (matches.length > 0) {
    return {calendar: matches[0], created: false};
  }

  var calendar = app.Calendar({name: name});
  app.calendars.push(calendar);
  return {calendar: app.calendars.whose({name: name})()[0], created: true};
}

function findEvent(calendar, eventURL) {
  var matches = calendar.events.whose({url: eventURL})();
  if (matches.length > 0) {
    return matches[0];
  }

  return null;
}

function applyEventFields(event, payload) {
  var desiredStart = new Date(payload.start_time_utc);
  var desiredEnd = new Date(payload.end_time_utc);
  var currentStart = event.startDate();
  var currentEnd = event.endDate();
  var expandedStart = currentStart < desiredStart ? currentStart : desiredStart;
  var expandedEnd = currentEnd > desiredEnd ? currentEnd : desiredEnd;

  event.summary = payload.title;
  event.startDate = expandedStart;
  event.endDate = expandedEnd;
  event.startDate = desiredStart;
  event.endDate = desiredEnd;
  event.location = payload.location || '';
  event.description = payload.notes || '';
  event.url = payload.url;
}

function eventMatches(event, payload) {
  if (event === null) {
    return false;
  }

  return event.summary() === payload.title &&
    event.location() === (payload.location || '') &&
    event.description() === (payload.notes || '') &&
    event.url() === payload.url &&
    event.startDate().getTime() === new Date(payload.start_time_utc).getTime() &&
    event.endDate().getTime() === new Date(payload.end_time_utc).getTime();
}

function run() {
  var input = JSON.parse(ObjC.unwrap($.getenv('VAGARO_SYNC_INPUT')));
  var calendarApp = Application('Calendar');
  calendarApp.includeStandardAdditions = true;
  var calendarResult = findCalendar(calendarApp, input.calendar_name);
  var calendar = calendarResult.calendar;

  if (input.action === 'ensure_calendar') {
    return JSON.stringify({ok: true, created: calendarResult.created});
  }

  if (input.action === 'upsert_event') {
    var existing = findEvent(calendar, input.event.url);
    if (existing === null) {
      var created = calendarApp.Event({
        summary: input.event.title,
        startDate: new Date(input.event.start_time_utc),
        endDate: new Date(input.event.end_time_utc),
        location: input.event.location || '',
        description: input.event.notes || '',
        url: input.event.url
      });
      calendar.events.push(created);
    } else {
      applyEventFields(existing, input.event);
    }

    return JSON.stringify({ok: true, event_url: input.event.url});
  }

  if (input.action === 'has_event') {
    return JSON.stringify({ok: true, exists: findEvent(calendar, input.event_url) !== null});
  }

  if (input.action === 'event_matches') {
    return JSON.stringify({ok: true, matches: eventMatches(findEvent(calendar, input.event.url), input.event)});
  }

  if (input.action === 'delete_event') {
    var eventToDelete = findEvent(calendar, input.event_url);
    if (eventToDelete !== null) {
      eventToDelete.delete();
    }

    return JSON.stringify({ok: true});
  }

  throw new Error('unsupported action: ' + input.action);
}
