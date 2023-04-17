package analyzer

//go:generate stringer -type=RuleId -linecomment

type RuleId int

const (
	// once before
	applyDefaultSelectLimitId      RuleId = iota // applyDefaultSelectLimit
	validateOffsetAndLimitId                     // validateOffsetAndLimit
	validateStarExpressionsId                    // validateStarExpressions
	validateCreateTableId                        // validateCreateTable
	validateExprSemId                            // validateExprSem
	resolveVariablesId                           // resolveVariables
	resolveNamedWindowsId                        // resolveNamedWindows
	resolveSetVariablesId                        // resolveSetVariables
	resolveViewsId                               // resolveViews
	liftCtesId                                   // liftCtes
	resolveCtesId                                // resolveCtes
	liftRecursiveCtesId                          // liftRecursiveCtes
	resolveDatabasesId                           // resolveDatabases
	resolveTablesId                              // resolveTables
	loadStoredProceduresId                       // loadStoredProcedures
	validateDropTablesId                         // validateDropTables
	setTargetSchemasId                           // setTargetSchemas
	resolveCreateLikeId                          // resolveCreateLike
	parseColumnDefaultsId                        // parseColumnDefaults
	resolveDropConstraintId                      // resolveDropConstraint
	validateDropConstraintId                     // validateDropConstraint
	loadCheckConstraintsId                       // loadCheckConstraints
	assignCatalogId                              // assignCatalog
	resolveAnalyzeTablesId                       // resolveAnalyzeTables
	resolveCreateSelectId                        // resolveCreateSelect
	resolveSubqueriesId                          // resolveSubqueries
	setViewTargetSchemaId                        // setViewTargetSchema
	resolveUnionsId                              // resolveUnions
	resolveDescribeQueryId                       // resolveDescribeQuery
	checkUniqueTableNamesId                      // checkUniqueTableNames
	disambiguateTableFunctionsId                 // disambiguateTableFunctions
	resolveTableFunctionsId                      // resolveTableFunctions
	resolveDeclarationsId                        // resolveDeclarations
	resolveColumnDefaultsId                      // resolveColumnDefaults
	validateColumnDefaultsId                     // validateColumnDefaults
	validateCreateTriggerId                      // validateCreateTrigger
	validateCreateProcedureId                    // validateCreateProcedure
	loadInfoSchemaId                             // loadInfoSchema
	validateDatabaseSetId                        // validateDatabaseSet
	validatePrivilegesId                         // validatePrivileges
	reresolveTablesId                            // reresolveTables
	setInsertColumnsId                           // setInsertColumns
	validateJoinComplexityId                     // validateJoinComplexity
	applyBinlogReplicaControllerId               // applyBinlogReplicaController

	// default
	resolveNaturalJoinsId          // resolveNaturalJoins
	resolveOrderbyLiteralsId       // resolveOrderbyLiterals
	resolveFunctionsId             // resolveFunctions
	flattenTableAliasesId          // flattenTableAliases
	pushdownSortId                 // pushdownSort
	pushdownGroupbyAliasesId       // pushdownGroupbyAliases
	pushdownSubqueryAliasFiltersId // pushdownSubqueryAliasFilters
	qualifyColumnsId               // qualifyColumns
	resolveColumnsId               // resolveColumns
	validateCheckConstraintId      // validateCheckConstraint
	resolveBarewordSetVariablesId  // resolveBarewordSetVariables
	replaceCountStarId             // replaceCountStar
	expandStarsId                  // expandStars
	transposeRightJoinsId          // transposeRightJoins
	resolveHavingId                // resolveHaving
	mergeUnionSchemasId            // mergeUnionSchemas
	flattenAggregationExprsId      // flattenAggregationExprs
	reorderProjectionId            // reorderProjection
	resolveSubqueryExprsId         // resolveSubqueryExprs
	replaceCrossJoinsId            // replaceCrossJoins
	moveJoinCondsToFilterId        // moveJoinCondsToFilter
	evalFilterId                   // evalFilter
	optimizeDistinctId             // optimizeDistinct

	// after default
	hoistOutOfScopeFiltersId     // hoistOutOfScopeFilters
	transformJoinApplyId         // transformJoinApply
	hoistSelectExistsId          // hoistSelectExists
	finalizeSubqueriesId         // finalizeSubqueries
	finalizeUnionsId             // finalizeUnions
	loadTriggersId               // loadTriggers
	loadEventsId                 // loadEvents
	processTruncateId            // processTruncate
	resolveAlterColumnId         // resolveAlterColumn
	resolveGeneratorsId          // resolveGenerators
	removeUnnecessaryConvertsId  // removeUnnecessaryConverts
	pruneColumnsId               // pruneColumns
	stripTableNameInDefaultsId   // stripTableNamesFromColumnDefaults
	foldEmptyJoinsId             // foldEmptyJoins
	optimizeJoinsId              // optimizeJoins
	pushdownFiltersId            // pushdownFilters
	subqueryIndexesId            // subqueryIndexes
	pruneTablesId                // pruneTables
	setJoinScopeLenId            // setJoinScopeLen
	eraseProjectionId            // eraseProjection
	replaceSortPkId              // replaceSortPk
	insertTopNId                 // insertTopN
	applyHashInId                // applyHashIn
	resolveInsertRowsId          // resolveInsertRows
	resolvePreparedInsertId      // resolvePreparedInsert
	applyTriggersId              // applyTriggers
	applyProceduresId            // applyProcedures
	assignRoutinesId             // assignRoutines
	modifyUpdateExprsForJoinId   // modifyUpdateExprsForJoin
	applyRowUpdateAccumulatorsId // applyRowUpdateAccumulators
	wrapWithRollbackId           // rollback triggers
	applyFKsId                   // applyFKs

	// validate
	validateAfterId  // validateAfter
	validateBeforeId // validateBefore

	// after all
	cacheSubqueryResultsId        // cacheSubqueryResults
	cacheSubqueryAliasesInJoinsId // cacheSubqueryAliasesInJoins
	AutocommitId                  // addAutocommitNode
	TrackProcessId                // trackProcess
	parallelizeId                 // parallelize
	clearWarningsId               // clearWarnings
)
