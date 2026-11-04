"""Exception hierarchy.

Every failure the operator must act on has a dedicated type: the CLI and the
control API translate them into exit codes and HTTP statuses without guessing
from a message string.
"""


class ScraperError(Exception):
    """Base class for every error raised by this package."""


class ConfigError(ScraperError):
    """Environment or path configuration is unusable."""


class SpecError(ScraperError):
    """A store spec is missing, malformed or internally inconsistent."""

    def __init__(self, path: str, message: str):
        self.path = path
        self.message = message
        super().__init__(f"{path}: {message}")


class MissingSpecError(SpecError):
    """No spec claims the URL that is about to be crawled."""


class AmbiguousSpecError(SpecError):
    """More than one spec claims the same URL."""


class StoreMismatchError(ScraperError):
    """DATA_SOURCE.store_id and the spec disagree about the store."""


class EmptyListingError(ScraperError):
    """A listing page produced zero items while the spec forbids that."""


class ExtractionError(ScraperError):
    """The document could not be parsed at all (not a single bad field)."""


class RunConflict(ScraperError):
    """A run is already active; starting another would duplicate work."""


class RunNotFound(ScraperError):
    """The requested run id does not exist."""


class TaskError(ScraperError):
    """A recurring task definition is malformed or cannot be applied."""


class TaskNotFound(ScraperError):
    """The requested task id does not exist."""


class StopTimeout(ScraperError):
    """A run ignored the stop request for longer than the grace period."""


class IngestionError(ScraperError):
    """The Ingestion API rejected a request or could not be reached."""

    def __init__(self, message: str, *, status: int | None = None) -> None:
        self.status = status
        prefix = f"HTTP {status} " if status else ""
        super().__init__(f"{prefix}{message}")
