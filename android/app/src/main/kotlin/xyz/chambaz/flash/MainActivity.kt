package xyz.chambaz.flash

import android.os.Bundle
import androidx.activity.compose.setContent
import androidx.appcompat.app.AppCompatActivity
import androidx.compose.runtime.*
import androidx.core.view.WindowCompat
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.coroutines.launch
import xyz.chambaz.flash.api.FlashApi
import xyz.chambaz.flash.model.CardState
import xyz.chambaz.flash.model.DayActivity
import xyz.chambaz.flash.model.DeckItem
import xyz.chambaz.flash.model.EvaluateResult
import xyz.chambaz.flash.store.Store
import xyz.chambaz.flash.ui.*

enum class Screen { Login, Settings, DeckList, Study, Done, Stats }

class MainActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        WindowCompat.setDecorFitsSystemWindows(window, false)

        val store = Store(this)

        setContent {
            var theme by remember { mutableStateOf(store.loadTheme()) }
            var accentIndex by remember { mutableIntStateOf(store.loadAccentIndex()) }

            FlashTheme(theme = theme, accentIndex = accentIndex) {
                val scope = rememberCoroutineScope()

                var screen by remember { mutableStateOf(if (store.hasConfig()) Screen.DeckList else Screen.Login) }

                var decks by remember { mutableStateOf<List<DeckItem>>(store.loadCachedDecks()) }
                var isRefreshing by remember { mutableStateOf(false) }

                var studyDeck by remember { mutableStateOf("") }
                var cards by remember { mutableStateOf<List<CardState>>(emptyList()) }
                var cardIndex by remember { mutableIntStateOf(0) }
                var deckTotal by remember { mutableIntStateOf(0) }
                var studyState by remember { mutableStateOf(StudyState.Question) }
                var evalResult by remember { mutableStateOf<EvaluateResult?>(null) }
                var evalError by remember { mutableStateOf<String?>(null) }
                var submittedAnswer by remember { mutableStateOf("") }
                var reviewedCount by remember { mutableIntStateOf(0) }

                var activity by remember { mutableStateOf<List<DayActivity>>(emptyList()) }
                var isLoadingActivity by remember { mutableStateOf(false) }

                fun api() = FlashApi(store.baseUrl(), store.loadToken())

                fun loadDecks(showIndicator: Boolean = false) {
                    scope.launch {
                        if (showIndicator) isRefreshing = true
                        try {
                            val fetched = withContext(Dispatchers.IO) { api().listDecks() }
                            decks = fetched
                            store.saveDecks(fetched)
                        } catch (_: Exception) {}
                        isRefreshing = false
                    }
                }

                LaunchedEffect(screen) {
                    when (screen) {
                        Screen.DeckList -> loadDecks(showIndicator = false)
                        Screen.Stats -> {
                            isLoadingActivity = true
                            try {
                                activity = withContext(Dispatchers.IO) { api().activity() }
                            } catch (_: Exception) {}
                            isLoadingActivity = false
                        }
                        else -> {}
                    }
                }

                fun startStudy(deck: DeckItem) {
                    scope.launch {
                        studyDeck = deck.name
                        studyState = StudyState.Question
                        evalResult = null
                        evalError = null
                        reviewedCount = 0
                        try {
                            val a = api()
                            val dueCards = withContext(Dispatchers.IO) { a.dueCards(deck.name) }
                            val total = withContext(Dispatchers.IO) { a.deckTotal(deck.name) }
                            cards = dueCards
                            deckTotal = total
                            cardIndex = 0
                            screen = Screen.Study
                        } catch (_: Exception) {}
                    }
                }

                fun submitAnswer(answer: String) {
                    val card = cards.getOrNull(cardIndex) ?: return
                    submittedAnswer = answer
                    studyState = StudyState.Evaluating
                    evalResult = null
                    evalError = null
                    scope.launch {
                        try {
                            val result = withContext(Dispatchers.IO) {
                                api().evaluate(studyDeck, card.id, answer, store.loadStep(), store.loadThreshold())
                            }
                            if (result.reshowInSession) {
                                cards = cards + card
                            }
                            evalResult = result
                            reviewedCount++
                        } catch (e: Exception) {
                            evalError = "Error: ${e.message}"
                        }
                        studyState = StudyState.Result
                    }
                }

                fun continueStudy() {
                    val next = cardIndex + 1
                    if (next >= cards.size) {
                        screen = Screen.Done
                    } else {
                        cardIndex = next
                        studyState = StudyState.Question
                        evalResult = null
                        evalError = null
                    }
                }

                when (screen) {
                    Screen.Login -> LoginScreen(
                        onConnect = { url, token ->
                            val testApi = FlashApi(url.trimEnd('/'), token)
                            withContext(Dispatchers.IO) { testApi.listDecks() }
                            store.saveUrl(url.trimEnd('/'))
                            store.saveToken(token)
                            screen = Screen.DeckList
                        },
                    )
                    Screen.Settings -> SettingsScreen(
                        initialUrl = store.loadUrl(),
                        initialToken = store.loadToken(),
                        initialTheme = theme,
                        initialAccentIndex = accentIndex,
                        initialThreshold = store.loadThreshold(),
                        initialStep = store.loadStep(),
                        onBack = { screen = Screen.DeckList },
                        onSave = { url, token ->
                            val testApi = FlashApi(url.trimEnd('/'), token)
                            withContext(Dispatchers.IO) { testApi.listDecks() }
                            store.saveUrl(url.trimEnd('/'))
                            store.saveToken(token)
                        },
                        onThemeChange = { t -> theme = t; store.saveTheme(t) },
                        onAccentChange = { i -> accentIndex = i; store.saveAccentIndex(i) },
                        onThresholdChange = { store.saveThreshold(it) },
                        onStepChange = { store.saveStep(it) },
                    )
                    Screen.DeckList -> DeckListScreen(
                        decks = decks,
                        isRefreshing = isRefreshing,
                        onRefresh = { loadDecks(showIndicator = true) },
                        onDeckSelected = { startStudy(it) },
                        onStats = { screen = Screen.Stats },
                        onSettings = { screen = Screen.Settings },
                    )
                    Screen.Study -> StudyScreen(
                        deckName = studyDeck,
                        cards = cards,
                        cardIndex = cardIndex,
                        deckTotal = deckTotal,
                        state = studyState,
                        result = evalResult,
                        error = evalError,
                        submittedAnswer = submittedAnswer,
                        threshold = store.loadThreshold(),
                        onSubmit = { submitAnswer(it) },
                        onContinue = { continueStudy() },
                        onBack = { screen = Screen.DeckList },
                    )
                    Screen.Done -> DoneScreen(
                        reviewedCount = reviewedCount,
                        onBack = { screen = Screen.DeckList },
                    )
                    Screen.Stats -> StatsScreen(
                        activity = activity,
                        isLoading = isLoadingActivity,
                        onBack = { screen = Screen.DeckList },
                    )
                }
            }
        }
    }
}
