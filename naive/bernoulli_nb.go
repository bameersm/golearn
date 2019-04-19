package naive

import (
	"fmt"
	"github.com/bameersm/golearn/base"
	"math"
)

// A Bernoulli Naive Bayes Classifier. Naive Bayes classifiers assumes
// that features probabilities are independent. In order to classify an
// instance, it is calculated the probability that it was generated by
// each known class, that is, for each class C, the following
// probability is calculated.
//
// p(C|F1, F2, F3... Fn)
//
// Being F1, F2... Fn the instance features. Using the bayes theorem
// this can be written as:
//
// \frac{p(C) \times p(F1, F2... Fn|C)}{p(F1, F2... Fn)}
//
// In the Bernoulli Naive Bayes features are considered independent
// booleans, this means that the likelihood of a document given a class
// C is given by:
//
// p(F1, F2... Fn) =
// \prod_{i=1}^{n}{[F_i \times p(f_i|C)) + (1-F_i)(1 - p(f_i|C)))]}
//
// where
//     - F_i equals to 1 if feature is present in vector and zero
//       otherwise
//     - p(f_i|C) the probability of class C generating the feature
//       f_i
//
// For more information:
//
// C.D. Manning, P. Raghavan and H. Schuetze (2008). Introduction to
// Information Retrieval. Cambridge University Press, pp. 234-265.
// http://nlp.stanford.edu/IR-book/html/htmledition/the-bernoulli-model-1.html
type BernoulliNBClassifier struct {
	base.BaseEstimator
	// Conditional probability for each term. This vector should be
	// accessed in the following way: p(f|c) = condProb[c][f].
	// Logarithm is used in order to avoid underflow.
	condProb map[string][]float64
	// Number of instances in each class. This is necessary in order to
	// calculate the laplace smooth value during the Predict step.
	classInstances map[string]int
	// Number of instances used in training.
	trainingInstances int
	// Number of features used in training
	features int
	// Attributes used to Train
	attrs []base.Attribute
	// Instance template
	fitOn base.FixedDataGrid
}

func (nb *BernoulliNBClassifier) GetMetadata() base.ClassifierMetadataV1 {
	return base.ClassifierMetadataV1{
		FormatVersion:      1,
		ClassifierName:     "KNN",
		ClassifierVersion:  "1.0",
		ClassifierMetadata: nil,
	}
}

func (nb *BernoulliNBClassifier) Save(filePath string) error {
	writer, err := base.CreateSerializedClassifierStub(filePath, nb.GetMetadata())
	if err != nil {
		return err
	}
	err = nb.SaveWithPrefix(writer, "")
	writer.Close()
	return err
}

func (nb *BernoulliNBClassifier) Load(filePath string) error {
	reader, err := base.ReadSerializedClassifierStub(filePath)
	if err != nil {
		return err
	}

	return nb.LoadWithPrefix(reader, "")
}

func (nb *BernoulliNBClassifier) LoadWithPrefix(reader *base.ClassifierDeserializer, prefix string) error {

	instances, err := reader.GetInstancesForKey(reader.Prefix(prefix, "INSTANCE_STRUCTURE"))
	if err != nil {
		return base.DescribeError("Unable to read INSTANCE_STRUCTURE", err)
	}

	rawAttrs, err := reader.GetAttributesForKey(reader.Prefix(prefix, "TRAINING_ATTRIBUTES"))
	if err != nil {
		return base.DescribeError("Unable to read training attributes", err)
	}
	attrs, err := base.ReplaceDeserializedAttributesWithVersionsFromInstances(rawAttrs, instances)
	if err != nil {
		return base.DescribeError("Unable to match up attributes", err)
	}

	numFeatures, err := reader.GetU64ForKey(reader.Prefix(prefix, "NUM_FEATURES"))
	if err != nil {
		return base.DescribeError("Unable to read training feature count", err)
	}
	numTrainingInstances, err := reader.GetU64ForKey(reader.Prefix(prefix, "NUM_TRAINING_INSTANCES"))
	if err != nil {
		return base.DescribeError("Unable to read training feature count", err)
	}

	// Save the class instances map
	condProbs := make(map[string][]float64)
	classInstances := make(map[string]int)

	err = reader.GetJSONForKey(reader.Prefix(prefix, "CLASS_INSTANCES"), &classInstances)
	if err != nil {
		return base.DescribeError("Unable to read the number of things in each class", err)
	}
	err = reader.GetJSONForKey(reader.Prefix(prefix, "COND_MAP"), &condProbs)
	if err != nil {
		return base.DescribeError("Unable to read the number of things in each class", err)
	}

	nb.fitOn = instances
	nb.attrs = attrs
	nb.features = int(numFeatures)
	nb.trainingInstances = int(numTrainingInstances)
	nb.classInstances = classInstances
	nb.condProb = condProbs
	return nil
}

func (nb *BernoulliNBClassifier) SaveWithPrefix(writer *base.ClassifierSerializer, prefix string) error {

	// Save the instance template
	err := writer.WriteInstancesForKey(writer.Prefix(prefix, "INSTANCE_STRUCTURE"), nb.fitOn, false)
	if err != nil {
		return base.DescribeError("Unable to write INSTANCE_STRUCTURE", err)
	}

	// Save the attributes used to train
	err = writer.WriteAttributesForKey(writer.Prefix(prefix, "TRAINING_ATTRIBUTES"), nb.attrs)
	if err != nil {
		return base.DescribeError("Unable to write training attributes", err)
	}

	// Save the number of features
	err = writer.WriteU64ForKey(writer.Prefix(prefix, "NUM_FEATURES"), uint64(nb.features))
	if err != nil {
		return base.DescribeError("Unable to write training feature count", err)
	}

	// Save the number of instances
	err = writer.WriteU64ForKey(writer.Prefix(prefix, "NUM_TRAINING_INSTANCES"), uint64(nb.trainingInstances))
	if err != nil {
		return base.DescribeError("Unable to write training feature count", err)
	}

	// Save the class instances map
	err = writer.WriteJSONForKey(writer.Prefix(prefix, "CLASS_INSTANCES"), nb.classInstances)
	if err != nil {
		return base.DescribeError("Unable to save the number of things in each class", err)
	}

	err = writer.WriteJSONForKey(writer.Prefix(prefix, "COND_MAP"), nb.condProb)
	if err != nil {
		return base.DescribeError("Unable to save conditional probability map", err)
	}
	return nil
}

// Create a new Bernoulli Naive Bayes Classifier. The argument 'classes'
// is the number of possible labels in the classification task.
func NewBernoulliNBClassifier() *BernoulliNBClassifier {
	nb := BernoulliNBClassifier{}
	nb.condProb = make(map[string][]float64)
	nb.features = 0
	nb.trainingInstances = 0
	return &nb
}

// Fill data matrix with Bernoulli Naive Bayes model. All values
// necessary for calculating prior probability and p(f_i)
func (nb *BernoulliNBClassifier) Fit(X base.FixedDataGrid) {

	// Check that all Attributes are binary
	classAttrs := X.AllClassAttributes()
	allAttrs := X.AllAttributes()
	featAttrs := base.AttributeDifference(allAttrs, classAttrs)
	for i := range featAttrs {
		if _, ok := featAttrs[i].(*base.BinaryAttribute); !ok {
			panic(fmt.Sprintf("%v: Should be BinaryAttribute", featAttrs[i]))
		}
	}
	featAttrSpecs := base.ResolveAttributes(X, featAttrs)

	// Check that only one classAttribute is defined
	if len(classAttrs) != 1 {
		panic("Only one class Attribute can be used")
	}

	// Number of features and instances in this training set
	_, nb.trainingInstances = X.Size()
	nb.attrs = featAttrs
	nb.features = len(featAttrs)

	// Number of instances in class
	nb.classInstances = make(map[string]int)

	// Number of documents with given term (by class)
	docsContainingTerm := make(map[string][]int)

	// This algorithm could be vectorized after binarizing the data
	// matrix. Since mat doesn't have this function, a iterative
	// version is used.
	X.MapOverRows(featAttrSpecs, func(docVector [][]byte, r int) (bool, error) {
		class := base.GetClass(X, r)

		// increment number of instances in class
		t, ok := nb.classInstances[class]
		if !ok {
			t = 0
		}
		nb.classInstances[class] = t + 1

		for feat := 0; feat < len(docVector); feat++ {
			v := docVector[feat]
			// In Bernoulli Naive Bayes the presence and absence of
			// features are considered. All non-zero values are
			// treated as presence.
			if v[0] > 0 {
				// Update number of times this feature appeared within
				// given label.
				t, ok := docsContainingTerm[class]
				if !ok {
					t = make([]int, nb.features)
					docsContainingTerm[class] = t
				}
				t[feat] += 1
			}
		}
		return true, nil
	})

	// Pre-calculate conditional probabilities for each class
	for c, _ := range nb.classInstances {
		nb.condProb[c] = make([]float64, nb.features)
		for feat := 0; feat < nb.features; feat++ {
			classTerms, _ := docsContainingTerm[c]
			numDocs := classTerms[feat]
			docsInClass, _ := nb.classInstances[c]

			classCondProb, _ := nb.condProb[c]
			// Calculate conditional probability with laplace smoothing
			classCondProb[feat] = float64(numDocs+1) / float64(docsInClass+1)
		}
	}

	nb.fitOn = base.NewStructuralCopy(X)
}

// Use trained model to predict test vector's class. The following
// operation is used in order to score each class:
//
// classScore = log(p(c)) + \sum_{f}{log(p(f|c))}
//
// PredictOne returns the string that represents the predicted class.
//
// IMPORTANT: PredictOne panics if Fit was not called or if the
// document vector and train matrix have a different number of columns.
func (nb *BernoulliNBClassifier) PredictOne(vector [][]byte) string {
	if nb.features == 0 {
		panic("Fit should be called before predicting")
	}

	if len(vector) != nb.features {
		panic("Different dimensions in Train and Test sets")
	}

	// Currently only the predicted class is returned.
	bestScore := -math.MaxFloat64
	bestClass := ""

	for class, classCount := range nb.classInstances {
		// Init classScore with log(prior)
		classScore := math.Log((float64(classCount)) / float64(nb.trainingInstances))
		for f := 0; f < nb.features; f++ {
			if vector[f][0] > 0 {
				// Test document has feature c
				classScore += math.Log(nb.condProb[class][f])
			} else {
				if nb.condProb[class][f] == 1.0 {
					// special case when prob = 1.0, consider laplace
					// smooth
					classScore += math.Log(1.0 / float64(nb.classInstances[class]+1))
				} else {
					classScore += math.Log(1.0 - nb.condProb[class][f])
				}
			}
		}

		if classScore > bestScore {
			bestScore = classScore
			bestClass = class
		}
	}

	return bestClass
}

// Predict is just a wrapper for the PredictOne function.
//
// IMPORTANT: Predict panics if Fit was not called or if the
// document vector and train matrix have a different number of columns.
func (nb *BernoulliNBClassifier) Predict(what base.FixedDataGrid) (base.UpdatableDataGrid, error) {
	// Generate return vector
	ret := base.GeneratePredictionVector(what)

	// Get the features
	featAttrSpecs := base.ResolveAttributes(what, nb.attrs)

	what.MapOverRows(featAttrSpecs, func(row [][]byte, i int) (bool, error) {
		base.SetClass(ret, i, nb.PredictOne(row))
		return true, nil
	})

	return ret, nil
}
